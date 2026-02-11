// APEX.BUILD Managed Database Panel
// Comprehensive database management interface

import React, { useState, useEffect, useCallback } from 'react'
import {
  Database,
  Plus,
  Server,
  Activity,
  Shield,
  Trash2,
  RefreshCw,
  Terminal,
  ExternalLink,
  Lock,
  Eye,
  EyeOff,
  Copy,
  CheckCircle2,
  AlertCircle,
  Sparkles
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { ManagedDatabase, CreateDatabaseRequest, DatabaseType } from '@/types'
import apiService from '@/services/api'
import { Button, Badge, Loading, Card, Avatar } from '@/components/ui'
import { SQLConsole } from './SQLConsole'

interface DatabasePanelProps {
  projectId: number
  className?: string
}

export const DatabasePanel: React.FC<DatabasePanelProps> = ({
  projectId,
  className
}) => {
  const [databases, setDatabases] = useState<ManagedDatabase[]>([])
  const [selectedDb, setSelectedDb] = useState<ManagedDatabase | null>(null)
  const [view, setView] = useState<'list' | 'create' | 'detail' | 'console'>('list')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showCredentials, setShowCredentials] = useState(false)
  const [credentials, setCredentials] = useState<any>(null)
  const [copied, setCopied] = useState(false)

  // Fetch databases
  const fetchDatabases = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    try {
      const data = await apiService.getDatabases(projectId)
      setDatabases(data)
    } catch (err) {
      console.error('Failed to fetch databases:', err)
      setError('Failed to load databases')
    } finally {
      if (!silent) setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    fetchDatabases()
  }, [fetchDatabases])

  // Reset credentials display when switching databases
  useEffect(() => {
    setShowCredentials(false)
    setCredentials(null)
  }, [selectedDb?.id])

  // Poll for provisioning databases
  useEffect(() => {
    const hasProvisioning = databases.some(db => db.status === 'provisioning')
    if (hasProvisioning) {
      const interval = setInterval(() => fetchDatabases(true), 3000)
      return () => clearInterval(interval)
    }
  }, [databases, fetchDatabases])

  // Handle database creation
  const handleCreate = async (data: CreateDatabaseRequest) => {
    setLoading(true)
    try {
      await apiService.createDatabase(projectId, data)
      setView('list')
      fetchDatabases()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create database')
    } finally {
      setLoading(false)
    }
  }

  // Handle reveal credentials
  const handleReveal = async (dbId: number) => {
    if (showCredentials) {
      setShowCredentials(false)
      return
    }

    setLoading(true)
    try {
      const db = await apiService.getDatabase(projectId, dbId, true)
      setCredentials(db.credentials)
      setShowCredentials(true)
    } catch (err) {
      console.error('Failed to reveal credentials:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleResetCredentials = async (dbId: number) => {
    const confirmed = window.confirm('Reset database credentials? Existing connections will stop working.')
    if (!confirmed) return

    setLoading(true)
    setError(null)
    try {
      const newCredentials = await apiService.resetDatabaseCredentials(projectId, dbId)
      setCredentials(newCredentials)
      setShowCredentials(true)
      fetchDatabases(true)
    } catch (err) {
      console.error('Failed to reset credentials:', err)
      setError('Failed to reset credentials')
    } finally {
      setLoading(false)
    }
  }

  // Handle delete
  const handleDelete = async (dbId: number) => {
    if (!window.confirm('Are you sure you want to delete this database? All data will be permanently lost.')) return

    try {
      await apiService.deleteDatabase(projectId, dbId)
      if (selectedDb?.id === dbId) setSelectedDb(null)
      setView('list')
      fetchDatabases()
    } catch (err) {
      console.error('Failed to delete database:', err)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Render List View
  const renderList = () => (
    <div className="flex flex-col h-full">
      <div className="p-4 flex justify-between items-center border-b border-gray-800">
        <h3 className="text-white font-semibold">Managed Databases</h3>
        <Button
          size="sm"
          variant="primary"
          icon={<Plus size={14} />}
          onClick={() => setView('create')}
        >
          New Database
        </Button>
      </div>

      {error && (
        <div className="mx-4 mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
          {error}
        </div>
      )}

      <div className="flex-1 overflow-auto p-4 space-y-4">
        {loading && databases.length === 0 ? (
          <div className="flex justify-center py-8"><Loading /></div>
        ) : databases.length === 0 ? (
          <div className="text-center py-12 text-gray-500">
            <Database className="w-12 h-12 mx-auto mb-4 opacity-20" />
            <p>No databases provisioned for this project</p>
            <p className="text-xs text-gray-600 mt-1">
              New projects automatically get a PostgreSQL database
            </p>
            <Button
              variant="link"
              className="text-cyan-400 mt-2"
              onClick={() => setView('create')}
            >
              Create additional database
            </Button>
          </div>
        ) : (
          databases.map(db => (
            <Card
              key={db.id}
              variant="cyberpunk"
              className="cursor-pointer hover:border-cyan-500/50 transition-all group"
              onClick={() => {
                setSelectedDb(db)
                setView('detail')
              }}
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className={cn(
                    "w-10 h-10 rounded-lg flex items-center justify-center",
                    db.type === 'postgresql' ? "bg-blue-900/30 text-blue-400" :
                    db.type === 'redis' ? "bg-red-900/30 text-red-400" : "bg-gray-800 text-gray-400"
                  )}>
                    <Database size={20} />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h4 className="text-white font-medium">{db.name}</h4>
                      {db.is_auto_provisioned && (
                        <Badge variant="primary" size="xs" className="flex items-center gap-1">
                          <Sparkles size={10} />
                          Auto
                        </Badge>
                      )}
                    </div>
                    <div className="flex items-center gap-2 mt-1">
                      <Badge variant="outline" size="xs" className="uppercase">{db.type}</Badge>
                      <span className="text-[10px] text-gray-500">{db.host}</span>
                    </div>
                  </div>
                </div>
                <div className="flex flex-col items-end gap-2">
                  <Badge
                    variant={
                      db.status === 'active' ? 'success' :
                      db.status === 'provisioning' ? 'warning' : 'error'
                    }
                    size="xs"
                    className="capitalize"
                  >
                    {db.status === 'provisioning' && <RefreshCw size={10} className="animate-spin mr-1" />}
                    {db.status}
                  </Badge>
                  <div className="text-[10px] text-gray-500">
                    {db.storage_used_mb}MB / {db.max_storage_mb}MB
                  </div>
                </div>
              </div>
            </Card>
          ))
        )}
      </div>
    </div>
  )

  // Render Create View
  const renderCreate = () => (
    <div className="flex flex-col h-full p-6">
      <div className="mb-6">
        <h3 className="text-xl font-bold text-white mb-2">Create New Database</h3>
        <p className="text-sm text-gray-400">Provision a production-ready managed database instance.</p>
      </div>

      <form className="space-y-6" onSubmit={(e) => {
        e.preventDefault()
        const formData = new FormData(e.currentTarget)
        handleCreate({
          name: formData.get('name') as string,
          type: formData.get('type') as DatabaseType
        })
      }}>
        <div className="space-y-2">
          <label className="text-sm font-medium text-gray-300">Database Name</label>
          <input
            name="name"
            type="text"
            required
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:border-cyan-500 outline-none"
            placeholder="my-app-db"
          />
        </div>

        <div className="space-y-2">
          <label className="text-sm font-medium text-gray-300">Engine Type</label>
          <div className="grid grid-cols-2 gap-4">
            <label className="relative flex flex-col items-center p-4 bg-gray-900 border border-gray-700 rounded-lg cursor-pointer hover:border-cyan-500/50 has-[:checked]:border-cyan-500 has-[:checked]:bg-cyan-500/10">
              <input type="radio" name="type" value="postgresql" defaultChecked className="hidden" />
              <Database className="w-8 h-8 text-blue-400 mb-2" />
              <span className="text-sm font-bold text-white">PostgreSQL</span>
              <span className="text-[10px] text-gray-500 text-center mt-1">v15.0 - Relational DB</span>
            </label>
            <label className="relative flex flex-col items-center p-4 bg-gray-900 border border-gray-700 rounded-lg cursor-pointer hover:border-cyan-500/50 has-[:checked]:border-red-500 has-[:checked]:bg-red-500/10">
              <input type="radio" name="type" value="redis" className="hidden" />
              <Database className="w-8 h-8 text-red-400 mb-2" />
              <span className="text-sm font-bold text-white">Redis</span>
              <span className="text-[10px] text-gray-500 text-center mt-1">v7.0 - Cache / KV</span>
            </label>
          </div>
        </div>

        <div className="p-4 bg-cyan-950/20 border border-cyan-900/50 rounded-lg">
          <div className="flex gap-3">
            <Shield className="w-5 h-5 text-cyan-400 shrink-0" />
            <div className="text-xs text-cyan-200">
              <p className="font-bold mb-1">Security & Backups Included</p>
              <p>Instances are automatically backed up daily. Connection strings are encrypted at rest.</p>
            </div>
          </div>
        </div>

        <div className="flex gap-3 pt-4">
          <Button
            type="button"
            variant="ghost"
            className="flex-1"
            onClick={() => setView('list')}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            className="flex-1"
            loading={loading}
          >
            Create Instance
          </Button>
        </div>
      </form>
    </div>
  )

  // Render Detail View
  const renderDetail = () => {
    if (!selectedDb) return null
    return (
      <div className="flex flex-col h-full overflow-hidden">
        <div className="p-4 border-b border-gray-800 flex items-center justify-between bg-gray-900/50">
          <div className="flex items-center gap-3">
            <button onClick={() => setView('list')} className="p-1 hover:bg-gray-800 rounded transition-colors">
              <Plus className="w-4 h-4 text-gray-400 rotate-45" />
            </button>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="text-white font-semibold">{selectedDb.name}</h3>
                {selectedDb.is_auto_provisioned && (
                  <Badge variant="primary" size="xs" className="flex items-center gap-1">
                    <Sparkles size={10} />
                    Auto-provisioned
                  </Badge>
                )}
              </div>
              <Badge variant="outline" size="xs" className="uppercase">{selectedDb.type}</Badge>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {!selectedDb.is_auto_provisioned && (
              <Button
                size="sm"
                variant="ghost"
                icon={<Trash2 size={14} />}
                className="text-gray-500 hover:text-red-400"
                onClick={() => handleDelete(selectedDb.id)}
              />
            )}
          </div>
        </div>

        {error && (
          <div className="mx-4 mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
            {error}
          </div>
        )}

        <div className="flex-1 overflow-auto p-4 space-y-6">
          {/* Metrics Summary */}
          <div className="grid grid-cols-2 gap-4">
            <Card variant="cyberpunk" className="p-3">
              <div className="flex items-center gap-2 text-gray-400 mb-2">
                <Activity size={14} />
                <span className="text-xs">Connections</span>
              </div>
              <div className="text-xl font-bold text-white">
                {selectedDb.connection_count} <span className="text-xs text-gray-500 font-normal">/ {selectedDb.max_connections}</span>
              </div>
            </Card>
            <Card variant="cyberpunk" className="p-3">
              <div className="flex items-center gap-2 text-gray-400 mb-2">
                <Database size={14} />
                <span className="text-xs">Storage</span>
              </div>
              <div className="text-xl font-bold text-white">
                {selectedDb.storage_used_mb}MB <span className="text-xs text-gray-500 font-normal">/ {selectedDb.max_storage_mb}MB</span>
              </div>
            </Card>
          </div>

          {/* Connection Info */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold text-gray-500 uppercase tracking-wider">Connection Info</h4>
            <div className="space-y-2">
              <div className="p-3 bg-black rounded-lg border border-gray-800 font-mono text-xs">
                <div className="flex justify-between items-center mb-1">
                  <span className="text-gray-500">Host</span>
                  <div className="flex items-center gap-2">
                    <span className="text-cyan-400">{selectedDb.host}</span>
                    <button onClick={() => copyToClipboard(selectedDb.host)}><Copy size={12} className="text-gray-600 hover:text-gray-400" /></button>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-500">Port</span>
                  <span className="text-white">{selectedDb.port}</span>
                </div>
              </div>

              <div className="p-3 bg-black rounded-lg border border-gray-800 font-mono text-xs">
                <div className="flex justify-between items-center mb-2">
                  <span className="text-gray-500">Password</span>
                  <button
                    onClick={() => handleReveal(selectedDb.id)}
                    className="flex items-center gap-1 text-cyan-400 hover:text-cyan-300"
                  >
                    {showCredentials ? <EyeOff size={12} /> : <Eye size={12} />}
                    <span>{showCredentials ? 'Hide' : 'Reveal'}</span>
                  </button>
                </div>
                <div className="bg-gray-900 p-2 rounded flex justify-between items-center">
                  <span className="text-gray-400 truncate mr-2">
                    {showCredentials ? (credentials?.password || '••••••••') : '••••••••••••••••'}
                  </span>
                  {showCredentials && credentials?.password && (
                    <button onClick={() => copyToClipboard(credentials.password)}><Copy size={12} className="text-gray-600 hover:text-gray-400" /></button>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="space-y-2">
            <Button
              className="w-full justify-start"
              variant="outline"
              icon={<Terminal size={14} />}
              onClick={() => setView('console')}
              disabled={selectedDb.status !== 'active'}
            >
              Open SQL Console
            </Button>
            <Button
              className="w-full justify-start"
              variant="outline"
              icon={<RefreshCw size={14} />}
              disabled={selectedDb.status !== 'active'}
              onClick={() => handleResetCredentials(selectedDb.id)}
            >
              Reset Credentials
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // Render Main Content
  return (
    <div className={cn("h-full bg-gray-950 flex flex-col", className)}>
      {view === 'list' && renderList()}
      {view === 'create' && renderCreate()}
      {view === 'detail' && renderDetail()}
      {view === 'console' && selectedDb && (
        <div className="h-full flex flex-col">
          <div className="p-2 bg-gray-900 border-b border-gray-800 flex items-center justify-between px-4">
            <button
              onClick={() => setView('detail')}
              className="text-xs text-gray-400 hover:text-white flex items-center gap-1"
            >
              <Plus className="w-3 h-3 rotate-45" /> Back to Instance
            </button>
            <Badge size="xs" variant="outline">{selectedDb.name}</Badge>
          </div>
          <SQLConsole
            projectId={projectId}
            dbId={selectedDb.id}
            className="flex-1"
          />
        </div>
      )}
    </div>
  )
}
