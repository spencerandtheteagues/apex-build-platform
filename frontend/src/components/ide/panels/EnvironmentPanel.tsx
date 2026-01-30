// APEX.BUILD Environment Configuration Panel
// Nix-like reproducible environment system for project dependencies
// Provides Replit parity for development environment configuration

import React, { useState, useEffect, useCallback } from 'react'
import {
  Settings,
  Package,
  RefreshCw,
  Plus,
  Trash2,
  Check,
  AlertCircle,
  Terminal,
  Cpu,
  Box,
  Sparkles,
  ChevronDown,
  ChevronRight,
  Download,
  Wand2,
  Layers
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button, Badge, Loading, Card } from '@/components/ui'
import apiService from '@/services/api'

interface EnvironmentPanelProps {
  projectId: number
  className?: string
}

// Types matching backend
interface PackageDependency {
  name: string
  version?: string
  source?: string
}

interface EnvironmentConfig {
  language: string
  version: string
  packages: PackageDependency[]
  dev_packages: PackageDependency[]
  system: string[]
  env_vars: Record<string, string>
  build_command?: string
  start_command?: string
  install_command?: string
  options?: Record<string, any>
}

interface RuntimeInfo {
  id: string
  name: string
  description: string
  versions: string[]
  default: string
  package_manager: string
  icon: string
}

interface EnvironmentPreset {
  id: string
  name: string
  description: string
  language: string
  version: string
  packages: PackageDependency[]
  dev_packages: PackageDependency[]
  system: string[]
}

interface PackageInfo {
  name: string
  description: string
  category: string
}

export const EnvironmentPanel: React.FC<EnvironmentPanelProps> = ({
  projectId,
  className
}) => {
  const [config, setConfig] = useState<EnvironmentConfig | null>(null)
  const [runtimes, setRuntimes] = useState<RuntimeInfo[]>([])
  const [presets, setPresets] = useState<EnvironmentPreset[]>([])
  const [availablePackages, setAvailablePackages] = useState<PackageInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [view, setView] = useState<'config' | 'presets' | 'packages'>('config')
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    runtime: true,
    packages: true,
    devPackages: false,
    system: false,
    commands: false
  })

  // New package form state
  const [newPackage, setNewPackage] = useState({ name: '', version: '', isDev: false })
  const [newSystemPkg, setNewSystemPkg] = useState('')

  // Fetch environment config
  const fetchConfig = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiService.get(`/environment/project/${projectId}`)
      if (response.data?.data?.environment) {
        setConfig(response.data.data.environment)
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to load environment configuration')
    } finally {
      setLoading(false)
    }
  }, [projectId])

  // Fetch available runtimes
  const fetchRuntimes = useCallback(async () => {
    try {
      const response = await apiService.get('/environment/runtimes')
      if (response.data?.data?.runtimes) {
        setRuntimes(response.data.data.runtimes)
      }
    } catch (err) {
      console.error('Failed to fetch runtimes:', err)
    }
  }, [])

  // Fetch presets
  const fetchPresets = useCallback(async () => {
    try {
      const response = await apiService.get('/environment/presets')
      if (response.data?.data?.presets) {
        setPresets(response.data.data.presets)
      }
    } catch (err) {
      console.error('Failed to fetch presets:', err)
    }
  }, [])

  // Fetch packages for current runtime
  const fetchPackages = useCallback(async (runtime: string) => {
    try {
      const response = await apiService.get(`/environment/packages/${runtime}`)
      if (response.data?.data?.packages) {
        setAvailablePackages(response.data.data.packages)
      }
    } catch (err) {
      console.error('Failed to fetch packages:', err)
    }
  }, [])

  useEffect(() => {
    fetchConfig()
    fetchRuntimes()
    fetchPresets()
  }, [fetchConfig, fetchRuntimes, fetchPresets])

  useEffect(() => {
    if (config?.language) {
      fetchPackages(config.language)
    }
  }, [config?.language, fetchPackages])

  // Save configuration
  const saveConfig = async () => {
    if (!config) return
    setSaving(true)
    setError(null)
    setSuccess(null)
    try {
      await apiService.put(`/environment/project/${projectId}`, config)
      setSuccess('Environment configuration saved successfully')
      setTimeout(() => setSuccess(null), 3000)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to save configuration')
    } finally {
      setSaving(false)
    }
  }

  // Auto-detect environment
  const detectEnvironment = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiService.post(`/environment/project/${projectId}/detect`)
      if (response.data?.data?.detected) {
        setConfig(response.data.data.detected)
        setSuccess('Environment detected successfully')
        setTimeout(() => setSuccess(null), 3000)
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to detect environment')
    } finally {
      setLoading(false)
    }
  }

  // Apply preset
  const applyPreset = async (presetId: string) => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiService.post(`/environment/project/${projectId}/preset/${presetId}`)
      if (response.data?.data?.environment) {
        setConfig(response.data.data.environment)
        setSuccess(`Preset applied successfully`)
        setView('config')
        setTimeout(() => setSuccess(null), 3000)
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to apply preset')
    } finally {
      setLoading(false)
    }
  }

  // Update config field
  const updateConfig = (field: keyof EnvironmentConfig, value: any) => {
    if (!config) return
    setConfig({ ...config, [field]: value })
  }

  // Add package
  const addPackage = () => {
    if (!config || !newPackage.name) return
    const pkg: PackageDependency = {
      name: newPackage.name,
      version: newPackage.version || undefined
    }
    if (newPackage.isDev) {
      updateConfig('dev_packages', [...(config.dev_packages || []), pkg])
    } else {
      updateConfig('packages', [...(config.packages || []), pkg])
    }
    setNewPackage({ name: '', version: '', isDev: false })
  }

  // Remove package
  const removePackage = (name: string, isDev: boolean) => {
    if (!config) return
    if (isDev) {
      updateConfig('dev_packages', (config.dev_packages || []).filter(p => p.name !== name))
    } else {
      updateConfig('packages', (config.packages || []).filter(p => p.name !== name))
    }
  }

  // Add system package
  const addSystemPackage = () => {
    if (!config || !newSystemPkg) return
    updateConfig('system', [...(config.system || []), newSystemPkg])
    setNewSystemPkg('')
  }

  // Remove system package
  const removeSystemPackage = (name: string) => {
    if (!config) return
    updateConfig('system', (config.system || []).filter(p => p !== name))
  }

  // Toggle section
  const toggleSection = (section: string) => {
    setExpandedSections(prev => ({ ...prev, [section]: !prev[section] }))
  }

  // Quick add package from suggestions
  const quickAddPackage = (pkg: PackageInfo, isDev: boolean = false) => {
    if (!config) return
    const newPkg: PackageDependency = { name: pkg.name }
    if (isDev) {
      if (!(config.dev_packages || []).find(p => p.name === pkg.name)) {
        updateConfig('dev_packages', [...(config.dev_packages || []), newPkg])
      }
    } else {
      if (!(config.packages || []).find(p => p.name === pkg.name)) {
        updateConfig('packages', [...(config.packages || []), newPkg])
      }
    }
  }

  // Get runtime icon color
  const getRuntimeColor = (runtimeId: string): string => {
    const colors: Record<string, string> = {
      node: 'text-green-400',
      python: 'text-yellow-400',
      go: 'text-cyan-400',
      rust: 'text-orange-400',
      java: 'text-red-400',
      ruby: 'text-red-500',
      php: 'text-indigo-400',
      deno: 'text-white',
      bun: 'text-pink-400'
    }
    return colors[runtimeId] || 'text-gray-400'
  }

  // Render section header
  const renderSectionHeader = (
    section: string,
    title: string,
    icon: React.ReactNode,
    count?: number
  ) => (
    <button
      onClick={() => toggleSection(section)}
      className="w-full flex items-center justify-between p-3 bg-gray-800/50 hover:bg-gray-800 rounded-lg transition-colors"
    >
      <div className="flex items-center gap-2">
        {icon}
        <span className="text-sm font-medium text-white">{title}</span>
        {count !== undefined && count > 0 && (
          <Badge variant="primary" size="xs">{count}</Badge>
        )}
      </div>
      {expandedSections[section] ? (
        <ChevronDown size={14} className="text-gray-400" />
      ) : (
        <ChevronRight size={14} className="text-gray-400" />
      )}
    </button>
  )

  // Render config view
  const renderConfigView = () => {
    if (!config) return null

    const currentRuntime = runtimes.find(r => r.id === config.language)

    return (
      <div className="space-y-4">
        {/* Runtime Section */}
        <div>
          {renderSectionHeader('runtime', 'Runtime', <Cpu size={14} className="text-cyan-400" />)}
          {expandedSections.runtime && (
            <div className="mt-2 p-4 bg-gray-900/50 rounded-lg border border-gray-800 space-y-4">
              {/* Language Select */}
              <div className="space-y-2">
                <label className="text-xs font-medium text-gray-400">Language</label>
                <select
                  value={config.language}
                  onChange={(e) => {
                    updateConfig('language', e.target.value)
                    const runtime = runtimes.find(r => r.id === e.target.value)
                    if (runtime) {
                      updateConfig('version', runtime.default)
                    }
                  }}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-cyan-500 outline-none"
                >
                  {runtimes.map(runtime => (
                    <option key={runtime.id} value={runtime.id}>
                      {runtime.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Version Select */}
              <div className="space-y-2">
                <label className="text-xs font-medium text-gray-400">Version</label>
                <select
                  value={config.version}
                  onChange={(e) => updateConfig('version', e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-cyan-500 outline-none"
                >
                  {currentRuntime?.versions.map(version => (
                    <option key={version} value={version}>
                      {version} {version === currentRuntime.default && '(recommended)'}
                    </option>
                  ))}
                </select>
              </div>

              {currentRuntime && (
                <div className="text-xs text-gray-500">
                  Package manager: <span className="text-gray-400">{currentRuntime.package_manager}</span>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Packages Section */}
        <div>
          {renderSectionHeader(
            'packages',
            'Dependencies',
            <Package size={14} className="text-green-400" />,
            (config.packages || []).length
          )}
          {expandedSections.packages && (
            <div className="mt-2 p-4 bg-gray-900/50 rounded-lg border border-gray-800 space-y-3">
              {/* Package list */}
              <div className="space-y-2">
                {(config.packages || []).map((pkg, idx) => (
                  <div
                    key={`${pkg.name}-${idx}`}
                    className="flex items-center justify-between p-2 bg-gray-800 rounded-lg group"
                  >
                    <div className="flex items-center gap-2">
                      <Package size={12} className="text-gray-500" />
                      <span className="text-sm text-white">{pkg.name}</span>
                      {pkg.version && (
                        <Badge variant="outline" size="xs">{pkg.version}</Badge>
                      )}
                    </div>
                    <button
                      onClick={() => removePackage(pkg.name, false)}
                      className="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-900/50 rounded transition-all"
                    >
                      <Trash2 size={12} className="text-red-400" />
                    </button>
                  </div>
                ))}
                {(config.packages || []).length === 0 && (
                  <div className="text-xs text-gray-500 text-center py-2">
                    No dependencies added
                  </div>
                )}
              </div>

              {/* Add package form */}
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newPackage.name}
                  onChange={(e) => setNewPackage({ ...newPackage, name: e.target.value })}
                  placeholder="Package name"
                  className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:border-cyan-500 outline-none"
                  onKeyDown={(e) => e.key === 'Enter' && addPackage()}
                />
                <input
                  type="text"
                  value={newPackage.version}
                  onChange={(e) => setNewPackage({ ...newPackage, version: e.target.value })}
                  placeholder="Version"
                  className="w-24 bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:border-cyan-500 outline-none"
                  onKeyDown={(e) => e.key === 'Enter' && addPackage()}
                />
                <Button size="sm" variant="primary" onClick={addPackage}>
                  <Plus size={14} />
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* Dev Packages Section */}
        <div>
          {renderSectionHeader(
            'devPackages',
            'Dev Dependencies',
            <Terminal size={14} className="text-purple-400" />,
            (config.dev_packages || []).length
          )}
          {expandedSections.devPackages && (
            <div className="mt-2 p-4 bg-gray-900/50 rounded-lg border border-gray-800 space-y-3">
              {/* Dev package list */}
              <div className="space-y-2">
                {(config.dev_packages || []).map((pkg, idx) => (
                  <div
                    key={`${pkg.name}-${idx}`}
                    className="flex items-center justify-between p-2 bg-gray-800 rounded-lg group"
                  >
                    <div className="flex items-center gap-2">
                      <Terminal size={12} className="text-gray-500" />
                      <span className="text-sm text-white">{pkg.name}</span>
                      {pkg.version && (
                        <Badge variant="outline" size="xs">{pkg.version}</Badge>
                      )}
                      <Badge variant="secondary" size="xs">dev</Badge>
                    </div>
                    <button
                      onClick={() => removePackage(pkg.name, true)}
                      className="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-900/50 rounded transition-all"
                    >
                      <Trash2 size={12} className="text-red-400" />
                    </button>
                  </div>
                ))}
                {(config.dev_packages || []).length === 0 && (
                  <div className="text-xs text-gray-500 text-center py-2">
                    No dev dependencies added
                  </div>
                )}
              </div>

              {/* Add dev package form */}
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newPackage.isDev ? newPackage.name : ''}
                  onChange={(e) => setNewPackage({ name: e.target.value, version: '', isDev: true })}
                  placeholder="Dev package name"
                  className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:border-cyan-500 outline-none"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      setNewPackage({ ...newPackage, isDev: true })
                      addPackage()
                    }
                  }}
                />
                <Button
                  size="sm"
                  variant="primary"
                  onClick={() => {
                    setNewPackage({ ...newPackage, isDev: true })
                    addPackage()
                  }}
                >
                  <Plus size={14} />
                </Button>
              </div>
            </div>
          )}
        </div>

        {/* System Packages Section */}
        <div>
          {renderSectionHeader(
            'system',
            'System Tools',
            <Box size={14} className="text-orange-400" />,
            (config.system || []).length
          )}
          {expandedSections.system && (
            <div className="mt-2 p-4 bg-gray-900/50 rounded-lg border border-gray-800 space-y-3">
              {/* System package list */}
              <div className="flex flex-wrap gap-2">
                {(config.system || []).map((pkg, idx) => (
                  <div
                    key={`${pkg}-${idx}`}
                    className="flex items-center gap-1 px-2 py-1 bg-gray-800 rounded-lg group"
                  >
                    <span className="text-xs text-white">{pkg}</span>
                    <button
                      onClick={() => removeSystemPackage(pkg)}
                      className="opacity-0 group-hover:opacity-100 p-0.5 hover:bg-red-900/50 rounded transition-all"
                    >
                      <Trash2 size={10} className="text-red-400" />
                    </button>
                  </div>
                ))}
              </div>

              {/* Add system package form */}
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newSystemPkg}
                  onChange={(e) => setNewSystemPkg(e.target.value)}
                  placeholder="System tool (git, curl, ffmpeg...)"
                  className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:border-cyan-500 outline-none"
                  onKeyDown={(e) => e.key === 'Enter' && addSystemPackage()}
                />
                <Button size="sm" variant="primary" onClick={addSystemPackage}>
                  <Plus size={14} />
                </Button>
              </div>

              {/* Common system tools */}
              <div className="pt-2 border-t border-gray-800">
                <div className="text-xs text-gray-500 mb-2">Quick add:</div>
                <div className="flex flex-wrap gap-1">
                  {['git', 'curl', 'wget', 'ffmpeg', 'imagemagick', 'jq'].map(tool => (
                    <button
                      key={tool}
                      onClick={() => {
                        if (!(config.system || []).includes(tool)) {
                          updateConfig('system', [...(config.system || []), tool])
                        }
                      }}
                      disabled={(config.system || []).includes(tool)}
                      className={cn(
                        "px-2 py-0.5 text-xs rounded transition-colors",
                        (config.system || []).includes(tool)
                          ? "bg-gray-700 text-gray-500 cursor-not-allowed"
                          : "bg-gray-800 text-gray-300 hover:bg-gray-700"
                      )}
                    >
                      {tool}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Commands Section */}
        <div>
          {renderSectionHeader(
            'commands',
            'Build Commands',
            <Settings size={14} className="text-blue-400" />
          )}
          {expandedSections.commands && (
            <div className="mt-2 p-4 bg-gray-900/50 rounded-lg border border-gray-800 space-y-4">
              <div className="space-y-2">
                <label className="text-xs font-medium text-gray-400">Install Command</label>
                <input
                  type="text"
                  value={config.install_command || ''}
                  onChange={(e) => updateConfig('install_command', e.target.value)}
                  placeholder={config.language === 'node' ? 'npm install' : config.language === 'python' ? 'pip install -r requirements.txt' : ''}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:border-cyan-500 outline-none"
                />
              </div>
              <div className="space-y-2">
                <label className="text-xs font-medium text-gray-400">Build Command</label>
                <input
                  type="text"
                  value={config.build_command || ''}
                  onChange={(e) => updateConfig('build_command', e.target.value)}
                  placeholder={config.language === 'node' ? 'npm run build' : ''}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:border-cyan-500 outline-none"
                />
              </div>
              <div className="space-y-2">
                <label className="text-xs font-medium text-gray-400">Start Command</label>
                <input
                  type="text"
                  value={config.start_command || ''}
                  onChange={(e) => updateConfig('start_command', e.target.value)}
                  placeholder={config.language === 'node' ? 'npm start' : config.language === 'python' ? 'python main.py' : ''}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:border-cyan-500 outline-none"
                />
              </div>
            </div>
          )}
        </div>
      </div>
    )
  }

  // Render presets view
  const renderPresetsView = () => (
    <div className="space-y-3">
      <div className="text-xs text-gray-400 px-1">
        Apply a preset to quickly configure your environment
      </div>
      {presets.map(preset => (
        <Card
          key={preset.id}
          variant="cyberpunk"
          className="cursor-pointer hover:border-cyan-500/50 transition-all"
          onClick={() => applyPreset(preset.id)}
        >
          <div className="flex items-start justify-between">
            <div>
              <div className="flex items-center gap-2">
                <Layers size={14} className={getRuntimeColor(preset.language)} />
                <span className="text-sm font-medium text-white">{preset.name}</span>
              </div>
              <p className="text-xs text-gray-400 mt-1">{preset.description}</p>
              <div className="flex items-center gap-2 mt-2">
                <Badge variant="outline" size="xs">{preset.language} {preset.version}</Badge>
                <Badge variant="secondary" size="xs">
                  {preset.packages.length} packages
                </Badge>
              </div>
            </div>
            <Button size="xs" variant="ghost">
              <Download size={12} />
            </Button>
          </div>
        </Card>
      ))}
    </div>
  )

  // Render packages view (suggestions)
  const renderPackagesView = () => {
    const categories = [...new Set(availablePackages.map(p => p.category))]

    return (
      <div className="space-y-4">
        <div className="text-xs text-gray-400 px-1">
          Popular packages for {config?.language || 'your runtime'}
        </div>
        {categories.map(category => (
          <div key={category}>
            <div className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-2 px-1">
              {category}
            </div>
            <div className="space-y-1">
              {availablePackages
                .filter(p => p.category === category)
                .map(pkg => {
                  const isAdded = (config?.packages || []).some(p => p.name === pkg.name) ||
                                  (config?.dev_packages || []).some(p => p.name === pkg.name)
                  return (
                    <div
                      key={pkg.name}
                      className="flex items-center justify-between p-2 bg-gray-900/50 rounded-lg hover:bg-gray-800/50 transition-colors"
                    >
                      <div>
                        <div className="text-sm text-white">{pkg.name}</div>
                        <div className="text-xs text-gray-500">{pkg.description}</div>
                      </div>
                      <div className="flex gap-1">
                        {isAdded ? (
                          <Badge variant="success" size="xs">
                            <Check size={10} className="mr-1" />
                            Added
                          </Badge>
                        ) : (
                          <>
                            <Button
                              size="xs"
                              variant="ghost"
                              onClick={() => quickAddPackage(pkg, false)}
                              className="text-xs"
                            >
                              Add
                            </Button>
                            {['dev', 'testing'].includes(category) && (
                              <Button
                                size="xs"
                                variant="ghost"
                                onClick={() => quickAddPackage(pkg, true)}
                                className="text-xs text-purple-400"
                              >
                                Dev
                              </Button>
                            )}
                          </>
                        )}
                      </div>
                    </div>
                  )
                })}
            </div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className={cn("h-full bg-gray-950 flex flex-col", className)}>
      {/* Header */}
      <div className="p-4 border-b border-gray-800">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Settings size={16} className="text-cyan-400" />
            <h3 className="text-white font-semibold">Environment</h3>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="xs"
              variant="ghost"
              onClick={detectEnvironment}
              disabled={loading}
              title="Auto-detect environment"
            >
              <Wand2 size={12} />
            </Button>
            <Button
              size="xs"
              variant="ghost"
              onClick={fetchConfig}
              disabled={loading}
            >
              <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
            </Button>
          </div>
        </div>

        {/* View tabs */}
        <div className="flex gap-1 bg-gray-900 p-1 rounded-lg">
          {[
            { id: 'config', label: 'Config', icon: Settings },
            { id: 'presets', label: 'Presets', icon: Sparkles },
            { id: 'packages', label: 'Packages', icon: Package }
          ].map(tab => (
            <button
              key={tab.id}
              onClick={() => setView(tab.id as any)}
              className={cn(
                "flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors",
                view === tab.id
                  ? "bg-gray-800 text-white"
                  : "text-gray-400 hover:text-white"
              )}
            >
              <tab.icon size={12} />
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Messages */}
      {error && (
        <div className="mx-4 mt-4 p-3 bg-red-900/20 border border-red-500/30 rounded-lg flex items-center gap-2">
          <AlertCircle size={14} className="text-red-400" />
          <span className="text-xs text-red-200">{error}</span>
        </div>
      )}
      {success && (
        <div className="mx-4 mt-4 p-3 bg-green-900/20 border border-green-500/30 rounded-lg flex items-center gap-2">
          <Check size={14} className="text-green-400" />
          <span className="text-xs text-green-200">{success}</span>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-auto p-4">
        {loading && !config ? (
          <div className="flex justify-center py-8">
            <Loading />
          </div>
        ) : (
          <>
            {view === 'config' && renderConfigView()}
            {view === 'presets' && renderPresetsView()}
            {view === 'packages' && renderPackagesView()}
          </>
        )}
      </div>

      {/* Save button */}
      {view === 'config' && config && (
        <div className="p-4 border-t border-gray-800">
          <Button
            className="w-full"
            variant="primary"
            onClick={saveConfig}
            loading={saving}
          >
            <Check size={14} className="mr-2" />
            Save Environment
          </Button>
        </div>
      )}
    </div>
  )
}

export default EnvironmentPanel
