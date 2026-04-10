import React, { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import {
  apiService,
  ExternalDeployment,
  ExternalDeploymentConfig,
  ExternalDeploymentDatabaseConfig,
  ExternalDeploymentLog,
  ExternalDeploymentProvider,
} from '@/services/api'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import {
  AlertTriangle,
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  Database,
  ExternalLink,
  RefreshCw,
  Rocket,
  RotateCcw,
  Square,
  Terminal,
} from 'lucide-react'

export interface ExternalDeploymentPanelProps {
  projectId: number
  projectName: string
  className?: string
}

const INITIAL_CONFIG: ExternalDeploymentConfig = {
  project_id: 0,
  provider: 'railway',
  environment: 'production',
  branch: 'main',
}

const INITIAL_DATABASE_CONFIG: ExternalDeploymentDatabaseConfig = {
  provider: 'neon',
  pooled: false,
}

const readErrorMessage = (err: unknown, fallback: string) => {
  if (typeof err === 'object' && err && 'response' in err) {
    const response = (err as { response?: { data?: { error?: string } } }).response
    if (typeof response?.data?.error === 'string' && response.data.error.trim() !== '') {
      return response.data.error
    }
  }
  if (err instanceof Error && err.message.trim() !== '') {
    return err.message
  }
  return fallback
}

const isDeploymentActive = (deployment: ExternalDeployment | null) =>
  deployment?.status === 'pending' ||
  deployment?.status === 'preparing' ||
  deployment?.status === 'building' ||
  deployment?.status === 'deploying'

export const ExternalDeploymentPanel: React.FC<ExternalDeploymentPanelProps> = ({
  projectId,
  projectName,
  className,
}) => {
  const [providers, setProviders] = useState<ExternalDeploymentProvider[]>([])
  const [deployments, setDeployments] = useState<ExternalDeployment[]>([])
  const [activeDeployment, setActiveDeployment] = useState<ExternalDeployment | null>(null)
  const [logs, setLogs] = useState<ExternalDeploymentLog[]>([])
  const [loading, setLoading] = useState(true)
  const [deploying, setDeploying] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [showDatabaseConfig, setShowDatabaseConfig] = useState(false)
  const [showLogs, setShowLogs] = useState(false)
  const [copiedUrl, setCopiedUrl] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [config, setConfig] = useState<ExternalDeploymentConfig>({
    ...INITIAL_CONFIG,
    project_id: projectId,
  })

  useEffect(() => {
    setConfig((prev) => ({ ...prev, project_id: projectId }))
  }, [projectId])

  const loadState = async (options?: { silent?: boolean }) => {
    if (!options?.silent) {
      setLoading(true)
    } else {
      setRefreshing(true)
    }
    try {
      setError(null)
      const [providerList, history] = await Promise.all([
        apiService.getExternalDeploymentProviders(),
        apiService.getExternalDeploymentHistory(projectId),
      ])
      setProviders(providerList || [])
      setDeployments(history.deployments || [])

      const latest = (history.deployments || [])[0] || null
      setActiveDeployment((prev) => {
        if (!prev) return latest
        return (history.deployments || []).find((deployment) => deployment.id === prev.id) || latest
      })

      setConfig((prev) => {
        if (providerList.length === 0) return prev
        if (providerList.some((provider) => provider.id === prev.provider)) return prev
        return { ...prev, provider: providerList[0].id }
      })
    } catch (err) {
      console.error('Failed to load external deployment state:', err)
      setError('Failed to load deployment providers or history.')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    loadState()
  }, [projectId]) // eslint-disable-line react-hooks/exhaustive-deps -- project change should refresh deployment state.

  useEffect(() => {
    if (!showLogs || !activeDeployment) return
    let cancelled = false
    const fetchLogs = async () => {
      try {
        const nextLogs = await apiService.getExternalDeploymentLogs(activeDeployment.id, 200)
        if (!cancelled) {
          setLogs(nextLogs)
        }
      } catch (err) {
        console.error('Failed to load external deployment logs:', err)
      }
    }
    fetchLogs()

    if (!isDeploymentActive(activeDeployment)) {
      return () => {
        cancelled = true
      }
    }

    const interval = window.setInterval(fetchLogs, 5000)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [activeDeployment, showLogs])

  useEffect(() => {
    if (!isDeploymentActive(activeDeployment)) return
    const interval = window.setInterval(() => {
      loadState({ silent: true })
    }, 5000)
    return () => window.clearInterval(interval)
  }, [activeDeployment?.id, activeDeployment?.status]) // eslint-disable-line react-hooks/exhaustive-deps -- polling only depends on active deployment identity/status.

  const selectedProvider = providers.find((provider) => provider.id === config.provider) || providers[0] || null

  const updateConfig = (key: keyof ExternalDeploymentConfig, value: string) => {
    setConfig((prev) => ({ ...prev, [key]: value }))
  }

  const setDatabaseEnabled = (enabled: boolean) => {
    setConfig((prev) => ({
      ...prev,
      database: enabled ? { ...INITIAL_DATABASE_CONFIG, ...prev.database } : undefined,
    }))
    if (!enabled) {
      setShowDatabaseConfig(false)
    }
  }

  const updateDatabaseConfig = <K extends keyof ExternalDeploymentDatabaseConfig>(
    key: K,
    value: ExternalDeploymentDatabaseConfig[K]
  ) => {
    setConfig((prev) => ({
      ...prev,
      database: {
        ...INITIAL_DATABASE_CONFIG,
        ...prev.database,
        [key]: value,
      },
    }))
  }

  const startDeployment = async () => {
    setDeploying(true)
    try {
      setError(null)
      const payload: ExternalDeploymentConfig = {
        ...config,
        project_id: projectId,
        provider: (selectedProvider?.id || config.provider),
      }

      const cleanedPayload = Object.fromEntries(
        Object.entries(payload).filter(([, value]) => value !== '' && value !== undefined && value !== null)
      ) as ExternalDeploymentConfig
      if (payload.database) {
        const cleanedDatabase = Object.fromEntries(
          Object.entries(payload.database).filter(([, value]) => value !== '' && value !== undefined && value !== null)
        ) as ExternalDeploymentDatabaseConfig
        cleanedPayload.database = cleanedDatabase
      }

      const result = await apiService.startExternalDeployment(cleanedPayload)
      setActiveDeployment(result.deployment)
      setShowLogs(false)
      await loadState({ silent: true })
    } catch (err) {
      console.error('Failed to start external deployment:', err)
      setError(readErrorMessage(err, 'Failed to start external deployment.'))
    } finally {
      setDeploying(false)
    }
  }

  const redeploy = async (deploymentId: string) => {
    try {
      setError(null)
      const result = await apiService.redeployExternalDeployment(deploymentId)
      setActiveDeployment(result.deployment)
      await loadState({ silent: true })
    } catch (err) {
      console.error('Failed to redeploy:', err)
      setError(readErrorMessage(err, 'Failed to redeploy the selected build.'))
    }
  }

  const cancelDeployment = async (deploymentId: string) => {
    try {
      setError(null)
      await apiService.cancelExternalDeployment(deploymentId)
      await loadState({ silent: true })
    } catch (err) {
      console.error('Failed to cancel deployment:', err)
      setError(readErrorMessage(err, 'Failed to cancel deployment.'))
    }
  }

  const copyUrl = () => {
    if (!activeDeployment?.url) return
    navigator.clipboard.writeText(activeDeployment.url)
    setCopiedUrl(true)
    window.setTimeout(() => setCopiedUrl(false), 2000)
  }

  const getStatusBadge = (status: ExternalDeployment['status']) => {
    switch (status) {
      case 'live':
        return <Badge variant="success">Live</Badge>
      case 'failed':
        return <Badge variant="error">Failed</Badge>
      case 'deploying':
        return <Badge variant="info">Deploying</Badge>
      case 'building':
        return <Badge variant="warning">Building</Badge>
      case 'preparing':
        return <Badge variant="warning">Preparing</Badge>
      case 'cancelled':
        return <Badge variant="default">Cancelled</Badge>
      default:
        return <Badge variant="default">{status}</Badge>
    }
  }

  const formatDuration = (ms?: number) => {
    if (!ms) return '-'
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  if (loading) {
    return (
      <Card variant="cyberpunk" className={className}>
        <CardContent className="flex items-center justify-center py-10">
          <RefreshCw className="h-5 w-5 animate-spin text-cyan-400" />
        </CardContent>
      </Card>
    )
  }

  return (
    <div className={cn('space-y-4', className)}>
      <Card variant="cyberpunk">
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-cyan-500/15 p-2">
                <Rocket className="h-5 w-5 text-cyan-300" />
              </div>
              <div>
                <CardTitle>External Deploy</CardTitle>
                <CardDescription>
                  Ship {projectName} to Railway, Cloudflare Pages, Render, Vercel, or Netlify.
                </CardDescription>
              </div>
            </div>
            {activeDeployment && getStatusBadge(activeDeployment.status)}
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          {providers.length === 0 ? (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-100">
              No external deployment providers are configured on the backend. Add provider tokens and required CLIs to enable one-click deploys.
            </div>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-2">
                {providers.map((provider) => {
                  const active = selectedProvider?.id === provider.id
                  return (
                    <button
                      key={provider.id}
                      type="button"
                      onClick={() => setConfig((prev) => ({ ...prev, provider: provider.id }))}
                      className={cn(
                        'rounded-lg border px-3 py-3 text-left transition-colors',
                        active
                          ? 'border-cyan-400/60 bg-cyan-500/10'
                          : 'border-gray-700/70 bg-gray-800/40 hover:border-gray-600'
                      )}
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="min-w-0">
                          <div className="text-sm font-semibold text-white">{provider.name}</div>
                          <div className="mt-1 text-xs text-gray-400">{provider.description}</div>
                        </div>
                        {active && <Badge variant="info">Selected</Badge>}
                      </div>
                    </button>
                  )
                })}
              </div>

              {selectedProvider && (
                <div className="flex flex-wrap gap-1.5">
                  {selectedProvider.features.slice(0, 5).map((feature) => (
                    <Badge key={feature} variant="outline" size="xs">
                      {feature}
                    </Badge>
                  ))}
                </div>
              )}

              <div className="rounded-lg border border-gray-700/70 bg-gray-900/40 p-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 text-sm font-semibold text-white">
                      <Database className="h-4 w-4 text-emerald-300" />
                      Managed Database
                    </div>
                    <div className="mt-1 text-xs text-gray-400">
                      Optionally provision Neon Postgres and inject `DATABASE_URL` plus standard `PG*` runtime env vars.
                    </div>
                  </div>
                  <Button
                    variant={config.database ? 'primary' : 'ghost'}
                    size="xs"
                    onClick={() => setDatabaseEnabled(!config.database)}
                  >
                    {config.database ? 'Enabled' : 'Enable'}
                  </Button>
                </div>

                {config.database && (
                  <div className="mt-3 space-y-3">
                    <button
                      type="button"
                      onClick={() => setShowDatabaseConfig((prev) => !prev)}
                      className="flex w-full items-center justify-between rounded-lg border border-gray-700/70 bg-gray-800/35 px-3 py-2 text-sm text-gray-200"
                    >
                      <span>Neon Postgres Settings</span>
                      {showDatabaseConfig ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                    </button>

                    {showDatabaseConfig && (
                      <div className="grid grid-cols-1 gap-3">
                        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Project Name</label>
                            <input
                              value={config.database.project_name || ''}
                              onChange={(event) => updateDatabaseConfig('project_name', event.target.value)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                              placeholder={`${projectName.toLowerCase().replace(/\s+/g, '-')}-db`}
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Region</label>
                            <input
                              value={config.database.region_id || ''}
                              onChange={(event) => updateDatabaseConfig('region_id', event.target.value)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                              placeholder="aws-us-east-2"
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Branch</label>
                            <input
                              value={config.database.branch_name || ''}
                              onChange={(event) => updateDatabaseConfig('branch_name', event.target.value)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                              placeholder="main"
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Database</label>
                            <input
                              value={config.database.database_name || ''}
                              onChange={(event) => updateDatabaseConfig('database_name', event.target.value)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                              placeholder="app"
                            />
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Role</label>
                            <input
                              value={config.database.role_name || ''}
                              onChange={(event) => updateDatabaseConfig('role_name', event.target.value)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                              placeholder="app_owner"
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Postgres Version</label>
                            <select
                              value={config.database.pg_version ? String(config.database.pg_version) : ''}
                              onChange={(event) => updateDatabaseConfig('pg_version', event.target.value ? Number(event.target.value) : undefined)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                            >
                              <option value="">Default</option>
                              <option value="14">14</option>
                              <option value="15">15</option>
                              <option value="16">16</option>
                              <option value="17">17</option>
                            </select>
                          </div>
                          <div>
                            <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Org ID</label>
                            <input
                              value={config.database.org_id || ''}
                              onChange={(event) => updateDatabaseConfig('org_id', event.target.value)}
                              className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                              placeholder="org-..."
                            />
                          </div>
                          <label className="flex items-center gap-2 rounded-lg border border-gray-700 bg-gray-900/60 px-3 py-2 text-sm text-gray-200">
                            <input
                              type="checkbox"
                              checked={Boolean(config.database.pooled)}
                              onChange={(event) => updateDatabaseConfig('pooled', event.target.checked)}
                              className="rounded border-gray-600 bg-gray-900 text-cyan-400"
                            />
                            Use pooled connection URI
                          </label>
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="grid grid-cols-1 gap-3">
                <div>
                  <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Branch</label>
                  <input
                    value={config.branch || ''}
                    onChange={(event) => updateConfig('branch', event.target.value)}
                    className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                    placeholder="main"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Environment</label>
                  <select
                    value={config.environment || 'production'}
                    onChange={(event) => updateConfig('environment', event.target.value)}
                    className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                  >
                    <option value="production">Production</option>
                    <option value="preview">Preview</option>
                  </select>
                </div>
              </div>

              <button
                type="button"
                onClick={() => setShowAdvanced((prev) => !prev)}
                className="flex w-full items-center justify-between rounded-lg border border-gray-700/70 bg-gray-800/35 px-3 py-2 text-sm text-gray-200"
              >
                <span>Advanced Build Settings</span>
                {showAdvanced ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
              </button>

              {showAdvanced && (
                <div className="grid grid-cols-1 gap-3">
                  <div>
                    <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Root Directory</label>
                    <input
                      value={config.root_directory || ''}
                      onChange={(event) => updateConfig('root_directory', event.target.value)}
                      className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                      placeholder="apps/web"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Output Directory</label>
                    <input
                      value={config.output_dir || ''}
                      onChange={(event) => updateConfig('output_dir', event.target.value)}
                      className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                      placeholder="dist"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Build Command</label>
                    <input
                      value={config.build_command || ''}
                      onChange={(event) => updateConfig('build_command', event.target.value)}
                      className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                      placeholder="npm run build"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Install Command</label>
                    <input
                      value={config.install_cmd || ''}
                      onChange={(event) => updateConfig('install_cmd', event.target.value)}
                      className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                      placeholder="npm ci"
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Start Command</label>
                    <input
                      value={config.start_command || ''}
                      onChange={(event) => updateConfig('start_command', event.target.value)}
                      className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                      placeholder="npm start"
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Framework</label>
                      <input
                        value={config.framework || ''}
                        onChange={(event) => updateConfig('framework', event.target.value)}
                        className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                        placeholder="react"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Node Version</label>
                      <input
                        value={config.node_version || ''}
                        onChange={(event) => updateConfig('node_version', event.target.value)}
                        className="w-full rounded-lg border border-gray-700 bg-gray-900/70 px-3 py-2 text-sm text-white focus:border-cyan-400 focus:outline-none"
                        placeholder="20"
                      />
                    </div>
                  </div>
                </div>
              )}

              {error && (
                <div className="rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-200">
                  {error}
                </div>
              )}

              <div className="flex gap-2">
                <Button
                  variant="primary"
                  onClick={startDeployment}
                  disabled={deploying || providers.length === 0}
                  icon={deploying ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Rocket className="h-4 w-4" />}
                  className="flex-1"
                >
                  {deploying ? 'Deploying…' : 'Start External Deploy'}
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => loadState({ silent: true })}
                  disabled={refreshing}
                  icon={<RefreshCw className={cn('h-4 w-4', refreshing && 'animate-spin')} />}
                >
                  Refresh
                </Button>
              </div>

              {activeDeployment && (
                <div className="space-y-3 rounded-lg border border-gray-700/70 bg-gray-900/60 p-3">
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <div className="truncate text-sm font-semibold text-white">
                          {activeDeployment.provider.replace('_', ' ')}
                        </div>
                        {getStatusBadge(activeDeployment.status)}
                      </div>
                      <div className="mt-1 text-xs text-gray-400">
                        {activeDeployment.branch} • {activeDeployment.environment}
                      </div>
                    </div>
                    <div className="flex gap-1">
                      {activeDeployment.url && (
                        <>
                          <Button variant="ghost" size="xs" onClick={copyUrl} icon={copiedUrl ? <Check size={14} /> : <Copy size={14} />} />
                          <Button
                            variant="ghost"
                            size="xs"
                            onClick={() => window.open(activeDeployment.url, '_blank', 'noopener,noreferrer')}
                            icon={<ExternalLink size={14} />}
                          />
                        </>
                      )}
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => redeploy(activeDeployment.id)}
                        icon={<RotateCcw size={14} />}
                        title="Redeploy"
                      />
                      {isDeploymentActive(activeDeployment) && (
                        <Button
                          variant="ghost"
                          size="xs"
                          onClick={() => cancelDeployment(activeDeployment.id)}
                          icon={<Square size={14} />}
                          title="Cancel"
                        />
                      )}
                    </div>
                  </div>

                  {activeDeployment.url && (
                    <a
                      href={activeDeployment.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="block truncate text-sm text-cyan-300 hover:text-cyan-200"
                    >
                      {activeDeployment.url}
                    </a>
                  )}

                  {activeDeployment.metadata?.database_provider === 'neon' && (
                    <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-100">
                      Neon Postgres
                      {typeof activeDeployment.metadata?.neon_project_name === 'string' && activeDeployment.metadata.neon_project_name
                        ? ` • ${activeDeployment.metadata.neon_project_name}`
                        : ''}
                      {typeof activeDeployment.metadata?.neon_database_name === 'string' && activeDeployment.metadata.neon_database_name
                        ? ` / ${activeDeployment.metadata.neon_database_name}`
                        : ''}
                    </div>
                  )}

                  <div className="grid grid-cols-3 gap-2">
                    <div className="rounded-lg bg-gray-800/50 p-2 text-center">
                      <div className="text-[11px] uppercase tracking-[0.16em] text-gray-500">Build</div>
                      <div className="mt-1 text-sm font-medium text-white">{formatDuration(activeDeployment.build_time)}</div>
                    </div>
                    <div className="rounded-lg bg-gray-800/50 p-2 text-center">
                      <div className="text-[11px] uppercase tracking-[0.16em] text-gray-500">Deploy</div>
                      <div className="mt-1 text-sm font-medium text-white">{formatDuration(activeDeployment.deploy_time)}</div>
                    </div>
                    <div className="rounded-lg bg-gray-800/50 p-2 text-center">
                      <div className="text-[11px] uppercase tracking-[0.16em] text-gray-500">Total</div>
                      <div className="mt-1 text-sm font-medium text-white">{formatDuration(activeDeployment.total_time)}</div>
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={() => setShowLogs((prev) => !prev)}
                    className="flex w-full items-center justify-between rounded-lg border border-gray-700/70 bg-gray-800/35 px-3 py-2 text-sm text-gray-200"
                  >
                    <span className="flex items-center gap-2">
                      <Terminal className="h-4 w-4 text-cyan-300" />
                      Deployment Logs
                    </span>
                    {showLogs ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                  </button>

                  {showLogs && (
                    <div className="max-h-64 overflow-auto rounded-lg border border-gray-800 bg-black/45 p-3 font-mono text-xs text-gray-300">
                      {logs.length === 0 ? (
                        <div className="text-gray-500">No logs yet.</div>
                      ) : (
                        logs.map((log) => (
                          <div key={`${log.id}-${log.timestamp}`} className="mb-2 last:mb-0">
                            <div className="text-[10px] uppercase tracking-[0.16em] text-gray-500">
                              {log.level} {log.phase ? `• ${log.phase}` : ''}
                            </div>
                            <div className="whitespace-pre-wrap break-words">{log.message}</div>
                          </div>
                        ))
                      )}
                    </div>
                  )}
                </div>
              )}

              {deployments.length > 0 && (
                <div className="space-y-2">
                  <div className="text-xs font-medium uppercase tracking-[0.2em] text-gray-400">Recent External Deploys</div>
                  {deployments.slice(0, 5).map((deployment) => {
                    const isActive = activeDeployment?.id === deployment.id
                    return (
                      <button
                        key={deployment.id}
                        type="button"
                        onClick={() => {
                          setActiveDeployment(deployment)
                          setShowLogs(false)
                        }}
                        className={cn(
                          'w-full rounded-lg border px-3 py-3 text-left transition-colors',
                          isActive
                            ? 'border-cyan-400/50 bg-cyan-500/10'
                            : 'border-gray-700/70 bg-gray-800/35 hover:border-gray-600'
                        )}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="truncate text-sm font-medium text-white">{deployment.provider.replace('_', ' ')}</span>
                              {getStatusBadge(deployment.status)}
                            </div>
                            <div className="mt-1 text-xs text-gray-400">
                              {deployment.branch} • {new Date(deployment.created_at).toLocaleString()}
                            </div>
                            {deployment.error_message && (
                              <div className="mt-1 flex items-start gap-1 text-xs text-red-300">
                                <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
                                <span className="line-clamp-2">{deployment.error_message}</span>
                              </div>
                            )}
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default ExternalDeploymentPanel
