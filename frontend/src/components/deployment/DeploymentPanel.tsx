// APEX.BUILD Deployment Panel Component
// Native hosting deployment management with Always-On support

import React, { useState, useEffect } from 'react'
import { cn } from '@/lib/utils'
import {
  apiService,
  NativeDeployment,
  NativeDeploymentConfig,
  DeploymentLog,
} from '@/services/api'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { AlwaysOnToggle } from './AlwaysOnToggle'
import {
  Rocket,
  Globe,
  Activity,
  Clock,
  Server,
  RefreshCw,
  Play,
  Square,
  ExternalLink,
  Copy,
  Check,
  AlertTriangle,
  Terminal,
  Zap,
  Settings,
  ChevronDown,
  ChevronUp,
} from 'lucide-react'

export interface DeploymentPanelProps {
  projectId: number
  projectName: string
  className?: string
}

export const DeploymentPanel: React.FC<DeploymentPanelProps> = ({
  projectId,
  projectName,
  className,
}) => {
  const [deployments, setDeployments] = useState<NativeDeployment[]>([])
  const [activeDeployment, setActiveDeployment] = useState<NativeDeployment | null>(null)
  const [logs, setLogs] = useState<DeploymentLog[]>([])
  const [loading, setLoading] = useState(true)
  const [deploying, setDeploying] = useState(false)
  const [showConfig, setShowConfig] = useState(false)
  const [showLogs, setShowLogs] = useState(false)
  const [copied, setCopied] = useState(false)
  const [config, setConfig] = useState<NativeDeploymentConfig>({
    port: 3000,
    always_on: false,
    auto_scale: false,
    min_instances: 1,
    max_instances: 3,
    memory_limit: 512,
    cpu_limit: 500,
    health_check_path: '/health',
  })

  useEffect(() => {
    loadDeployments()
  }, [projectId])

  useEffect(() => {
    if (activeDeployment && showLogs) {
      loadLogs(activeDeployment.id)
      const interval = setInterval(() => loadLogs(activeDeployment.id), 5000)
      return () => clearInterval(interval)
    }
  }, [activeDeployment, showLogs])

  const loadDeployments = async () => {
    try {
      const data = await apiService.getNativeDeployments(projectId)
      setDeployments(data.deployments || [])
      if (data.deployments?.length > 0) {
        setActiveDeployment(data.deployments[0])
      }
    } catch (err) {
      console.error('Failed to load deployments:', err)
    } finally {
      setLoading(false)
    }
  }

  const loadLogs = async (deploymentId: string) => {
    try {
      const logsData = await apiService.getDeploymentLogs(projectId, deploymentId)
      setLogs(logsData)
    } catch (err) {
      console.error('Failed to load logs:', err)
    }
  }

  const startDeployment = async () => {
    setDeploying(true)
    try {
      const result = await apiService.startNativeDeployment(projectId, config)
      setActiveDeployment(result.deployment)
      await loadDeployments()
    } catch (err) {
      console.error('Failed to start deployment:', err)
    } finally {
      setDeploying(false)
    }
  }

  const stopDeployment = async () => {
    if (!activeDeployment) return
    try {
      await apiService.stopNativeDeployment(projectId, activeDeployment.id)
      await loadDeployments()
    } catch (err) {
      console.error('Failed to stop deployment:', err)
    }
  }

  const restartDeployment = async () => {
    if (!activeDeployment) return
    try {
      await apiService.restartNativeDeployment(projectId, activeDeployment.id)
      await loadDeployments()
    } catch (err) {
      console.error('Failed to restart deployment:', err)
    }
  }

  const copyUrl = () => {
    if (activeDeployment?.url) {
      navigator.clipboard.writeText(activeDeployment.url)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'running':
        return <Badge variant="success" icon={<Activity size={12} className="animate-pulse" />}>Running</Badge>
      case 'building':
        return <Badge variant="warning" icon={<RefreshCw size={12} className="animate-spin" />}>Building</Badge>
      case 'deploying':
        return <Badge variant="info" icon={<Rocket size={12} />}>Deploying</Badge>
      case 'stopped':
        return <Badge variant="default" icon={<Square size={12} />}>Stopped</Badge>
      case 'failed':
        return <Badge variant="error" icon={<AlertTriangle size={12} />}>Failed</Badge>
      default:
        return <Badge variant="default">{status}</Badge>
    }
  }

  const formatDuration = (ms?: number): string => {
    if (!ms) return '-'
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  if (loading) {
    return (
      <Card variant="cyberpunk" className={className}>
        <CardContent className="flex items-center justify-center py-12">
          <RefreshCw className="w-6 h-6 text-cyan-400 animate-spin" />
        </CardContent>
      </Card>
    )
  }

  return (
    <div className={cn('space-y-4', className)}>
      {/* Main Deployment Card */}
      <Card variant="cyberpunk">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-cyan-500/20 rounded-lg">
                <Rocket className="w-5 h-5 text-cyan-400" />
              </div>
              <div>
                <CardTitle>Native Hosting</CardTitle>
                <CardDescription>
                  Deploy to {projectName}.apex.app
                </CardDescription>
              </div>
            </div>
            {activeDeployment && getStatusBadge(activeDeployment.status)}
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Active Deployment Info */}
          {activeDeployment ? (
            <>
              {/* URL */}
              <div className="flex items-center gap-2 p-3 bg-gray-800/50 rounded-lg border border-gray-700/50">
                <Globe className="w-4 h-4 text-cyan-400 flex-shrink-0" />
                <a
                  href={activeDeployment.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-cyan-400 hover:text-cyan-300 truncate flex-1"
                >
                  {activeDeployment.url}
                </a>
                <button
                  onClick={copyUrl}
                  className="p-1 hover:bg-gray-700 rounded transition-colors"
                >
                  {copied ? (
                    <Check size={14} className="text-green-400" />
                  ) : (
                    <Copy size={14} className="text-gray-400" />
                  )}
                </button>
                <a
                  href={activeDeployment.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 hover:bg-gray-700 rounded transition-colors"
                >
                  <ExternalLink size={14} className="text-gray-400" />
                </a>
              </div>

              {/* Stats */}
              <div className="grid grid-cols-4 gap-2">
                <div className="p-2 bg-gray-800/50 rounded-lg text-center">
                  <div className="text-xs text-gray-400">Build</div>
                  <div className="text-sm font-medium text-white">
                    {formatDuration(activeDeployment.build_duration)}
                  </div>
                </div>
                <div className="p-2 bg-gray-800/50 rounded-lg text-center">
                  <div className="text-xs text-gray-400">Deploy</div>
                  <div className="text-sm font-medium text-white">
                    {formatDuration(activeDeployment.deploy_duration)}
                  </div>
                </div>
                <div className="p-2 bg-gray-800/50 rounded-lg text-center">
                  <div className="text-xs text-gray-400">Requests</div>
                  <div className="text-sm font-medium text-white">
                    {activeDeployment.total_requests.toLocaleString()}
                  </div>
                </div>
                <div className="p-2 bg-gray-800/50 rounded-lg text-center">
                  <div className="text-xs text-gray-400">Avg Latency</div>
                  <div className="text-sm font-medium text-white">
                    {activeDeployment.avg_response_time}ms
                  </div>
                </div>
              </div>

              {/* Actions */}
              <div className="flex items-center gap-2">
                {activeDeployment.status === 'running' ? (
                  <>
                    <Button
                      size="sm"
                      variant="ghost"
                      icon={<Square size={14} />}
                      onClick={stopDeployment}
                    >
                      Stop
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      icon={<RefreshCw size={14} />}
                      onClick={restartDeployment}
                    >
                      Restart
                    </Button>
                  </>
                ) : (
                  <Button
                    size="sm"
                    variant="primary"
                    icon={<Play size={14} />}
                    onClick={startDeployment}
                    loading={deploying}
                  >
                    Deploy
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  icon={<Terminal size={14} />}
                  onClick={() => setShowLogs(!showLogs)}
                >
                  {showLogs ? 'Hide Logs' : 'View Logs'}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  icon={<Settings size={14} />}
                  onClick={() => setShowConfig(!showConfig)}
                >
                  Config
                </Button>
              </div>
            </>
          ) : (
            /* No Deployment - Show Deploy Button */
            <div className="text-center py-6">
              <Server className="w-12 h-12 text-gray-600 mx-auto mb-3" />
              <p className="text-gray-400 mb-4">No active deployment</p>
              <Button
                variant="primary"
                icon={<Rocket size={16} />}
                onClick={() => setShowConfig(true)}
                loading={deploying}
              >
                Deploy to apex.app
              </Button>
            </div>
          )}
        </CardContent>

        {/* Configuration Panel */}
        {showConfig && (
          <CardContent className="border-t border-gray-700/50 pt-4">
            <div className="space-y-4">
              <h4 className="text-sm font-medium text-white flex items-center gap-2">
                <Settings size={14} />
                Deployment Configuration
              </h4>

              <div className="grid grid-cols-2 gap-3">
                {/* Port */}
                <div>
                  <label className="text-xs text-gray-400">Port</label>
                  <input
                    type="number"
                    value={config.port}
                    onChange={(e) => setConfig({ ...config, port: parseInt(e.target.value) })}
                    className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm focus:border-cyan-500 focus:outline-none"
                  />
                </div>

                {/* Memory Limit */}
                <div>
                  <label className="text-xs text-gray-400">Memory (MB)</label>
                  <input
                    type="number"
                    value={config.memory_limit}
                    onChange={(e) => setConfig({ ...config, memory_limit: parseInt(e.target.value) })}
                    className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm focus:border-cyan-500 focus:outline-none"
                  />
                </div>

                {/* Build Command */}
                <div className="col-span-2">
                  <label className="text-xs text-gray-400">Build Command</label>
                  <input
                    type="text"
                    value={config.build_command || ''}
                    onChange={(e) => setConfig({ ...config, build_command: e.target.value })}
                    placeholder="npm run build"
                    className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm focus:border-cyan-500 focus:outline-none"
                  />
                </div>

                {/* Start Command */}
                <div className="col-span-2">
                  <label className="text-xs text-gray-400">Start Command</label>
                  <input
                    type="text"
                    value={config.start_command || ''}
                    onChange={(e) => setConfig({ ...config, start_command: e.target.value })}
                    placeholder="npm start"
                    className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm focus:border-cyan-500 focus:outline-none"
                  />
                </div>

                {/* Always-On Toggle */}
                <div className="col-span-2 flex items-center justify-between p-3 bg-gray-800/50 rounded-lg">
                  <div className="flex items-center gap-2">
                    <Zap size={16} className={config.always_on ? 'text-cyan-400' : 'text-gray-500'} />
                    <span className="text-sm text-white">Always-On</span>
                    <span className="text-xs text-gray-400">(24/7 uptime)</span>
                  </div>
                  <button
                    onClick={() => setConfig({ ...config, always_on: !config.always_on })}
                    className={cn(
                      'relative w-12 h-6 rounded-full transition-colors',
                      config.always_on ? 'bg-cyan-500' : 'bg-gray-700'
                    )}
                  >
                    <span
                      className={cn(
                        'absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform',
                        config.always_on && 'translate-x-6'
                      )}
                    />
                  </button>
                </div>
              </div>

              <Button
                variant="primary"
                icon={<Rocket size={14} />}
                onClick={startDeployment}
                loading={deploying}
                className="w-full"
              >
                Deploy Now
              </Button>
            </div>
          </CardContent>
        )}

        {/* Logs Panel */}
        {showLogs && activeDeployment && (
          <CardContent className="border-t border-gray-700/50 pt-4">
            <div className="space-y-2">
              <h4 className="text-sm font-medium text-white flex items-center gap-2">
                <Terminal size={14} />
                Deployment Logs
              </h4>
              <div className="h-48 overflow-y-auto bg-gray-900 rounded-lg p-3 font-mono text-xs">
                {logs.length > 0 ? (
                  logs.map((log, i) => (
                    <div
                      key={i}
                      className={cn(
                        'py-0.5',
                        log.level === 'error' && 'text-red-400',
                        log.level === 'warn' && 'text-yellow-400',
                        log.level === 'info' && 'text-gray-300',
                        log.level === 'debug' && 'text-gray-500'
                      )}
                    >
                      <span className="text-gray-500">
                        [{new Date(log.timestamp).toLocaleTimeString()}]
                      </span>{' '}
                      <span className="text-cyan-400">[{log.source}]</span>{' '}
                      {log.message}
                    </div>
                  ))
                ) : (
                  <div className="text-gray-500 text-center py-8">
                    No logs available
                  </div>
                )}
              </div>
            </div>
          </CardContent>
        )}
      </Card>

      {/* Always-On Panel (when deployment is active) */}
      {activeDeployment && activeDeployment.status === 'running' && (
        <AlwaysOnToggle
          projectId={projectId}
          deploymentId={activeDeployment.id}
          initialEnabled={activeDeployment.always_on}
          onStatusChange={(enabled) => {
            setActiveDeployment({ ...activeDeployment, always_on: enabled })
          }}
        />
      )}
    </div>
  )
}

export default DeploymentPanel
