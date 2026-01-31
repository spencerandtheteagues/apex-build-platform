// APEX.BUILD Package Manager Panel
// Search, install, and manage packages across npm, PyPI, and Go registries

import React, { useState, useCallback, useEffect, useMemo } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Input } from '@/components/ui/Input'
import { Card } from '@/components/ui/Card'
import { Loading } from '@/components/ui/Loading'
import apiService, {
  PackageRegistry,
  PackageSearchResult,
  InstalledPackage,
  PackageSuggestion,
} from '@/services/api'
import {
  Package,
  Search,
  Download,
  Trash2,
  RefreshCw,
  Plus,
  Sparkles,
  ExternalLink,
  ChevronDown,
  ChevronRight,
  AlertCircle,
  Check,
  X,
  Lightbulb,
} from 'lucide-react'

export interface PackageManagerProps {
  projectId: number | undefined
  className?: string
}

type TabType = 'search' | 'installed' | 'suggestions'

interface RegistryOption {
  value: PackageRegistry
  label: string
  icon: string
}

const REGISTRY_OPTIONS: RegistryOption[] = [
  { value: 'npm', label: 'npm', icon: 'N' },
  { value: 'pip', label: 'PyPI', icon: 'P' },
  { value: 'go', label: 'Go', icon: 'G' },
]

export const PackageManager: React.FC<PackageManagerProps> = ({
  projectId,
  className,
}) => {
  // State
  const [activeTab, setActiveTab] = useState<TabType>('installed')
  const [selectedRegistry, setSelectedRegistry] = useState<PackageRegistry>('npm')
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<PackageSearchResult[]>([])
  const [installedPackages, setInstalledPackages] = useState<{
    npm?: InstalledPackage[]
    pip?: InstalledPackage[]
    go?: InstalledPackage[]
  }>({})
  const [suggestions, setSuggestions] = useState<PackageSuggestion[]>([])
  const [projectLanguage, setProjectLanguage] = useState<string>('')
  const [projectFramework, setProjectFramework] = useState<string>('')

  // Loading states
  const [isSearching, setIsSearching] = useState(false)
  const [isLoadingInstalled, setIsLoadingInstalled] = useState(false)
  const [isLoadingSuggestions, setIsLoadingSuggestions] = useState(false)
  const [installingPackages, setInstallingPackages] = useState<Set<string>>(new Set())
  const [uninstallingPackages, setUninstallingPackages] = useState<Set<string>>(new Set())
  const [isUpdatingAll, setIsUpdatingAll] = useState(false)

  // UI state
  const [error, setError] = useState<string | null>(null)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    npm: true,
    pip: true,
    go: true,
    suggestions: true,
  })
  const [showRegistryDropdown, setShowRegistryDropdown] = useState(false)

  // Load installed packages on mount and when projectId changes
  useEffect(() => {
    if (projectId) {
      loadInstalledPackages()
      loadSuggestions()
    }
  }, [projectId])

  // Load installed packages
  const loadInstalledPackages = useCallback(async () => {
    if (!projectId) return

    setIsLoadingInstalled(true)
    setError(null)

    try {
      const packages = await apiService.listProjectPackages(projectId)
      setInstalledPackages(packages)
    } catch (err: any) {
      console.error('Failed to load installed packages:', err)
      setError('Failed to load installed packages')
    } finally {
      setIsLoadingInstalled(false)
    }
  }, [projectId])

  // Load package suggestions
  const loadSuggestions = useCallback(async () => {
    if (!projectId) return

    setIsLoadingSuggestions(true)

    try {
      const result = await apiService.getPackageSuggestions(projectId)
      setSuggestions(result.suggestions)
      setProjectLanguage(result.language)
      setProjectFramework(result.framework)
    } catch (err: any) {
      console.error('Failed to load suggestions:', err)
    } finally {
      setIsLoadingSuggestions(false)
    }
  }, [projectId])

  // Search packages
  const handleSearch = useCallback(async () => {
    if (!searchQuery.trim()) {
      setSearchResults([])
      return
    }

    setIsSearching(true)
    setError(null)

    try {
      const results = await apiService.searchPackages(searchQuery, selectedRegistry)
      setSearchResults(results)
    } catch (err: any) {
      console.error('Failed to search packages:', err)
      setError('Failed to search packages')
      setSearchResults([])
    } finally {
      setIsSearching(false)
    }
  }, [searchQuery, selectedRegistry])

  // Debounced search
  useEffect(() => {
    if (activeTab !== 'search') return

    const timer = setTimeout(() => {
      if (searchQuery.trim()) {
        handleSearch()
      }
    }, 300)

    return () => clearTimeout(timer)
  }, [searchQuery, selectedRegistry, activeTab])

  // Install package
  const handleInstall = useCallback(async (
    packageName: string,
    version: string = 'latest',
    isDev: boolean = false,
    registry: PackageRegistry = selectedRegistry
  ) => {
    if (!projectId) return

    const key = `${registry}:${packageName}`
    setInstallingPackages(prev => new Set(prev).add(key))
    setError(null)

    try {
      await apiService.installPackage(projectId, packageName, version, registry, isDev)
      await loadInstalledPackages()
    } catch (err: any) {
      console.error('Failed to install package:', err)
      setError(`Failed to install ${packageName}`)
    } finally {
      setInstallingPackages(prev => {
        const next = new Set(prev)
        next.delete(key)
        return next
      })
    }
  }, [projectId, selectedRegistry, loadInstalledPackages])

  // Uninstall package
  const handleUninstall = useCallback(async (
    packageName: string,
    registry: PackageRegistry
  ) => {
    if (!projectId) return

    const key = `${registry}:${packageName}`
    setUninstallingPackages(prev => new Set(prev).add(key))
    setError(null)

    try {
      await apiService.uninstallPackage(projectId, packageName, registry)
      await loadInstalledPackages()
    } catch (err: any) {
      console.error('Failed to uninstall package:', err)
      setError(`Failed to uninstall ${packageName}`)
    } finally {
      setUninstallingPackages(prev => {
        const next = new Set(prev)
        next.delete(key)
        return next
      })
    }
  }, [projectId, loadInstalledPackages])

  // Update all packages
  const handleUpdateAll = useCallback(async () => {
    if (!projectId) return

    setIsUpdatingAll(true)
    setError(null)

    try {
      await apiService.updateAllPackages(projectId)
      await loadInstalledPackages()
    } catch (err: any) {
      console.error('Failed to update packages:', err)
      setError('Failed to update packages')
    } finally {
      setIsUpdatingAll(false)
    }
  }, [projectId, loadInstalledPackages])

  // Toggle section expansion
  const toggleSection = (key: string) => {
    setExpandedSections(prev => ({ ...prev, [key]: !prev[key] }))
  }

  // Get total installed package count
  const totalInstalled = useMemo(() => {
    return (installedPackages.npm?.length || 0) +
           (installedPackages.pip?.length || 0) +
           (installedPackages.go?.length || 0)
  }, [installedPackages])

  // Check if a package is installed
  const isPackageInstalled = useCallback((name: string, registry: PackageRegistry) => {
    const packages = installedPackages[registry] || []
    return packages.some(p => p.name === name)
  }, [installedPackages])

  // Render search results
  const renderSearchResult = (pkg: PackageSearchResult) => {
    const isInstalled = isPackageInstalled(pkg.name, selectedRegistry)
    const isInstalling = installingPackages.has(`${selectedRegistry}:${pkg.name}`)

    return (
      <div
        key={pkg.name}
        className="flex items-start gap-3 p-3 hover:bg-gray-800/50 transition-colors border-b border-gray-800/50 last:border-b-0"
      >
        <div className="w-8 h-8 rounded bg-gray-800 flex items-center justify-center shrink-0">
          <Package size={16} className="text-red-400" />
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-white truncate">{pkg.name}</span>
            <Badge variant="neutral" size="xs">{pkg.version}</Badge>
            {pkg.license && (
              <Badge variant="outline" size="xs">{pkg.license}</Badge>
            )}
          </div>

          {pkg.description && (
            <p className="text-xs text-gray-500 mt-1 line-clamp-2">{pkg.description}</p>
          )}

          {pkg.keywords && pkg.keywords.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-1.5">
              {pkg.keywords.slice(0, 4).map(kw => (
                <span key={kw} className="text-[10px] text-gray-600 bg-gray-800/50 px-1.5 py-0.5 rounded">
                  {kw}
                </span>
              ))}
            </div>
          )}
        </div>

        <div className="flex items-center gap-1 shrink-0">
          {pkg.homepage && (
            <a
              href={pkg.homepage}
              target="_blank"
              rel="noopener noreferrer"
              className="p-1.5 text-gray-500 hover:text-gray-300 transition-colors"
              title="View homepage"
            >
              <ExternalLink size={14} />
            </a>
          )}

          {isInstalled ? (
            <Badge variant="success" size="xs" icon={<Check size={10} />}>
              Installed
            </Badge>
          ) : (
            <Button
              size="xs"
              variant="primary"
              onClick={() => handleInstall(pkg.name, pkg.version)}
              loading={isInstalling}
              disabled={isInstalling}
              icon={<Download size={12} />}
            >
              Install
            </Button>
          )}
        </div>
      </div>
    )
  }

  // Render installed package
  const renderInstalledPackage = (pkg: InstalledPackage, registry: PackageRegistry) => {
    const isUninstalling = uninstallingPackages.has(`${registry}:${pkg.name}`)

    return (
      <div
        key={`${registry}:${pkg.name}`}
        className="flex items-center gap-3 px-3 py-2 hover:bg-gray-800/30 transition-colors group"
      >
        <Package size={14} className="text-gray-500 shrink-0" />

        <span className="text-sm text-gray-300 flex-1 truncate" title={pkg.name}>
          {pkg.name}
        </span>

        <Badge variant="neutral" size="xs">{pkg.version}</Badge>

        {pkg.is_dev && (
          <Badge variant="warning" size="xs">dev</Badge>
        )}

        <button
          onClick={() => handleUninstall(pkg.name, registry)}
          disabled={isUninstalling}
          className={cn(
            'p-1 text-gray-600 hover:text-red-400 transition-colors opacity-0 group-hover:opacity-100',
            isUninstalling && 'opacity-100 cursor-not-allowed'
          )}
          title="Uninstall package"
        >
          {isUninstalling ? (
            <Loading size="xs" variant="spinner" />
          ) : (
            <Trash2 size={12} />
          )}
        </button>
      </div>
    )
  }

  // Render installed section for a registry
  const renderInstalledSection = (registry: PackageRegistry, label: string) => {
    const packages = installedPackages[registry] || []
    const isExpanded = expandedSections[registry]

    if (packages.length === 0) return null

    return (
      <div key={registry} className="border-b border-gray-800/50 last:border-b-0">
        <button
          onClick={() => toggleSection(registry)}
          className="w-full flex items-center gap-2 px-3 py-2 hover:bg-gray-800/30 transition-colors"
        >
          {isExpanded ? (
            <ChevronDown size={12} className="text-gray-500" />
          ) : (
            <ChevronRight size={12} className="text-gray-500" />
          )}
          <span className="text-xs font-medium text-gray-400 uppercase tracking-wide">
            {label}
          </span>
          <Badge variant="outline" size="xs">{packages.length}</Badge>
        </button>

        {isExpanded && (
          <div className="pb-1">
            {packages.map(pkg => renderInstalledPackage(pkg, registry))}
          </div>
        )}
      </div>
    )
  }

  // Render suggestion
  const renderSuggestion = (suggestion: PackageSuggestion) => {
    const registry = getRegistryForLanguage(projectLanguage)
    const isInstalled = isPackageInstalled(suggestion.name, registry)
    const isInstalling = installingPackages.has(`${registry}:${suggestion.name}`)

    return (
      <div
        key={suggestion.name}
        className="flex items-center gap-3 px-3 py-2 hover:bg-gray-800/30 transition-colors"
      >
        <Lightbulb size={14} className="text-yellow-500/70 shrink-0" />

        <div className="flex-1 min-w-0">
          <span className="text-sm text-gray-300">{suggestion.name}</span>
          <p className="text-xs text-gray-600 truncate">{suggestion.description}</p>
        </div>

        <Badge variant="neutral" size="xs">{suggestion.category}</Badge>

        {suggestion.is_dev && (
          <Badge variant="warning" size="xs">dev</Badge>
        )}

        {isInstalled ? (
          <Check size={14} className="text-green-500" />
        ) : (
          <Button
            size="xs"
            variant="ghost"
            onClick={() => handleInstall(suggestion.name, 'latest', suggestion.is_dev, registry)}
            loading={isInstalling}
            disabled={isInstalling}
            icon={<Plus size={12} />}
          >
            Add
          </Button>
        )}
      </div>
    )
  }

  // Get registry for project language
  const getRegistryForLanguage = (language: string): PackageRegistry => {
    switch (language.toLowerCase()) {
      case 'python':
        return 'pip'
      case 'go':
        return 'go'
      default:
        return 'npm'
    }
  }

  if (!projectId) {
    return (
      <div className={cn('flex flex-col h-full bg-gray-900/80 items-center justify-center p-6', className)}>
        <Package className="w-10 h-10 text-gray-600 mb-3" />
        <p className="text-sm text-gray-500 text-center">
          Select a project to manage packages
        </p>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col h-full bg-gray-900/80', className)}>
      {/* Header */}
      <div className="p-3 border-b border-gray-700/50">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <Package size={16} className="text-red-400" />
            <span className="text-sm font-medium text-white">Packages</span>
            {totalInstalled > 0 && (
              <Badge variant="outline" size="xs">{totalInstalled}</Badge>
            )}
          </div>

          <div className="flex items-center gap-1">
            <Button
              size="xs"
              variant="ghost"
              onClick={loadInstalledPackages}
              loading={isLoadingInstalled}
              icon={<RefreshCw size={12} />}
              title="Refresh"
              className="text-gray-400 hover:text-white"
            />
            {totalInstalled > 0 && (
              <Button
                size="xs"
                variant="ghost"
                onClick={handleUpdateAll}
                loading={isUpdatingAll}
                icon={<Sparkles size={12} />}
                title="Update all packages"
                className="text-gray-400 hover:text-white"
              >
                Update All
              </Button>
            )}
          </div>
        </div>

        {/* Tabs */}
        <div className="flex border border-gray-700 rounded-lg overflow-hidden">
          <button
            onClick={() => setActiveTab('installed')}
            className={cn(
              'flex-1 px-3 py-1.5 text-xs font-medium transition-colors',
              activeTab === 'installed'
                ? 'bg-red-500/20 text-red-400 border-r border-gray-700'
                : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800/50 border-r border-gray-700'
            )}
          >
            Installed
          </button>
          <button
            onClick={() => setActiveTab('search')}
            className={cn(
              'flex-1 px-3 py-1.5 text-xs font-medium transition-colors',
              activeTab === 'search'
                ? 'bg-red-500/20 text-red-400 border-r border-gray-700'
                : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800/50 border-r border-gray-700'
            )}
          >
            Search
          </button>
          <button
            onClick={() => setActiveTab('suggestions')}
            className={cn(
              'flex-1 px-3 py-1.5 text-xs font-medium transition-colors',
              activeTab === 'suggestions'
                ? 'bg-red-500/20 text-red-400'
                : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800/50'
            )}
          >
            Suggestions
          </button>
        </div>
      </div>

      {/* Error display */}
      {error && (
        <div className="px-3 py-2">
          <div className="bg-red-500/10 border border-red-500/30 rounded p-2 flex items-start gap-2">
            <AlertCircle size={14} className="text-red-400 mt-0.5 shrink-0" />
            <p className="text-xs text-red-400 flex-1">{error}</p>
            <button
              onClick={() => setError(null)}
              className="text-red-400 hover:text-red-300"
            >
              <X size={12} />
            </button>
          </div>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {/* Search Tab */}
        {activeTab === 'search' && (
          <div className="flex flex-col h-full">
            {/* Search input and registry selector */}
            <div className="p-3 space-y-2">
              <div className="flex gap-2">
                <div className="flex-1 relative">
                  <Input
                    placeholder="Search packages..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleSearch()
                    }}
                    leftIcon={<Search size={14} />}
                    size="sm"
                    variant="default"
                  />
                </div>

                {/* Registry selector */}
                <div className="relative">
                  <button
                    onClick={() => setShowRegistryDropdown(!showRegistryDropdown)}
                    className="h-9 px-3 bg-gray-800 border border-gray-600 rounded-lg flex items-center gap-2 hover:border-gray-500 transition-colors"
                  >
                    <span className="text-sm text-white">
                      {REGISTRY_OPTIONS.find(r => r.value === selectedRegistry)?.label}
                    </span>
                    <ChevronDown size={12} className="text-gray-500" />
                  </button>

                  {showRegistryDropdown && (
                    <div className="absolute top-full right-0 mt-1 bg-gray-800 border border-gray-600 rounded-lg shadow-xl z-20 min-w-[100px] overflow-hidden">
                      {REGISTRY_OPTIONS.map(option => (
                        <button
                          key={option.value}
                          onClick={() => {
                            setSelectedRegistry(option.value)
                            setShowRegistryDropdown(false)
                            setSearchResults([])
                          }}
                          className={cn(
                            'w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-gray-700 transition-colors text-left',
                            selectedRegistry === option.value && 'bg-gray-700/50 text-red-400'
                          )}
                        >
                          <span className="w-5 h-5 rounded bg-gray-900 flex items-center justify-center text-xs font-bold text-gray-400">
                            {option.icon}
                          </span>
                          <span className={selectedRegistry === option.value ? 'text-red-400' : 'text-gray-300'}>
                            {option.label}
                          </span>
                          {selectedRegistry === option.value && (
                            <Check size={12} className="ml-auto text-green-400" />
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Search results */}
            <div className="flex-1 overflow-y-auto">
              {isSearching ? (
                <div className="flex items-center justify-center py-8">
                  <Loading size="md" variant="spinner" />
                </div>
              ) : searchResults.length > 0 ? (
                <div>
                  {searchResults.map(renderSearchResult)}
                </div>
              ) : searchQuery.trim() ? (
                <div className="flex flex-col items-center justify-center py-8 text-gray-500">
                  <Package size={32} className="mb-2 opacity-50" />
                  <p className="text-sm">No packages found</p>
                  <p className="text-xs text-gray-600 mt-1">Try a different search term</p>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-8 text-gray-500">
                  <Search size={32} className="mb-2 opacity-50" />
                  <p className="text-sm">Search for packages</p>
                  <p className="text-xs text-gray-600 mt-1">
                    Type to search the {REGISTRY_OPTIONS.find(r => r.value === selectedRegistry)?.label} registry
                  </p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Installed Tab */}
        {activeTab === 'installed' && (
          <div>
            {isLoadingInstalled ? (
              <div className="flex items-center justify-center py-8">
                <Loading size="md" variant="spinner" />
              </div>
            ) : totalInstalled > 0 ? (
              <div>
                {renderInstalledSection('npm', 'npm')}
                {renderInstalledSection('pip', 'PyPI')}
                {renderInstalledSection('go', 'Go')}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-8 text-gray-500">
                <Package size={32} className="mb-2 opacity-50" />
                <p className="text-sm">No packages installed</p>
                <p className="text-xs text-gray-600 mt-1">
                  Search for packages to install them
                </p>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setActiveTab('search')}
                  icon={<Search size={14} />}
                  className="mt-3"
                >
                  Search Packages
                </Button>
              </div>
            )}
          </div>
        )}

        {/* Suggestions Tab */}
        {activeTab === 'suggestions' && (
          <div>
            {isLoadingSuggestions ? (
              <div className="flex items-center justify-center py-8">
                <Loading size="md" variant="spinner" />
              </div>
            ) : suggestions.length > 0 ? (
              <div>
                {/* Project info */}
                <div className="px-3 py-2 border-b border-gray-800/50">
                  <p className="text-xs text-gray-500">
                    Suggestions for{' '}
                    <span className="text-gray-400">{projectLanguage}</span>
                    {projectFramework && (
                      <>
                        {' / '}
                        <span className="text-gray-400">{projectFramework}</span>
                      </>
                    )}
                  </p>
                </div>

                {/* Suggestion categories */}
                {(() => {
                  const categories = [...new Set(suggestions.map(s => s.category))]
                  return categories.map(category => {
                    const categorySuggestions = suggestions.filter(s => s.category === category)
                    const isExpanded = expandedSections[`suggestion_${category}`] !== false

                    return (
                      <div key={category} className="border-b border-gray-800/50 last:border-b-0">
                        <button
                          onClick={() => toggleSection(`suggestion_${category}`)}
                          className="w-full flex items-center gap-2 px-3 py-2 hover:bg-gray-800/30 transition-colors"
                        >
                          {isExpanded ? (
                            <ChevronDown size={12} className="text-gray-500" />
                          ) : (
                            <ChevronRight size={12} className="text-gray-500" />
                          )}
                          <span className="text-xs font-medium text-gray-400 uppercase tracking-wide">
                            {category}
                          </span>
                          <Badge variant="outline" size="xs">{categorySuggestions.length}</Badge>
                        </button>

                        {isExpanded && (
                          <div className="pb-1">
                            {categorySuggestions.map(renderSuggestion)}
                          </div>
                        )}
                      </div>
                    )
                  })
                })()}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center py-8 text-gray-500">
                <Lightbulb size={32} className="mb-2 opacity-50" />
                <p className="text-sm">No suggestions available</p>
                <p className="text-xs text-gray-600 mt-1">
                  Suggestions are based on your project type
                </p>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default PackageManager
