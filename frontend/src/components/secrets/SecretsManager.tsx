import React, { useState, useEffect, useCallback } from 'react'
import { Eye, EyeOff, Plus, Trash2, RefreshCw, Key, Database, Lock, Unlock, AlertCircle, Shield, Copy, Check } from 'lucide-react'
import api from '../../services/api'

interface Secret {
  id: number
  name: string
  description?: string
  type: 'api_key' | 'database' | 'oauth' | 'environment' | 'ssh' | 'generic'
  project_id?: number
  last_accessed?: string
  rotation_due?: string
  created_at: string
  updated_at: string
}

interface SecretsManagerProps {
  projectId?: number
  onSelect?: (name: string, value: string) => void
}

const secretTypeIcons: Record<string, React.ReactNode> = {
  api_key: <Key className="w-4 h-4" />,
  database: <Database className="w-4 h-4" />,
  oauth: <Shield className="w-4 h-4" />,
  environment: <Lock className="w-4 h-4" />,
  ssh: <Unlock className="w-4 h-4" />,
  generic: <Key className="w-4 h-4" />,
}

const secretTypeLabels: Record<string, string> = {
  api_key: 'API Key',
  database: 'Database',
  oauth: 'OAuth',
  environment: 'Environment',
  ssh: 'SSH',
  generic: 'Generic',
}

export default function SecretsManager({ projectId, onSelect }: SecretsManagerProps) {
  const [secrets, setSecrets] = useState<Secret[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [revealedSecrets, setRevealedSecrets] = useState<Set<number>>(new Set())
  const [secretValues, setSecretValues] = useState<Record<number, string>>({})
  const [copied, setCopied] = useState<number | null>(null)

  // Form state
  const [newSecret, setNewSecret] = useState({
    name: '',
    value: '',
    description: '',
    type: 'environment' as Secret['type'],
  })

  const fetchSecrets = useCallback(async () => {
    try {
      setLoading(true)
      const params = projectId ? { project_id: projectId } : {}
      const response = await api.get('/secrets', { params })
      setSecrets(response.data.secrets || [])
      setError(null)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch secrets')
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    fetchSecrets()
  }, [fetchSecrets])

  const createSecret = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/secrets', {
        ...newSecret,
        project_id: projectId,
      })
      setNewSecret({ name: '', value: '', description: '', type: 'environment' })
      setShowCreateForm(false)
      fetchSecrets()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create secret')
    }
  }

  const deleteSecret = async (id: number) => {
    if (!confirm('Are you sure you want to delete this secret? This cannot be undone.')) return
    try {
      await api.delete(`/secrets/${id}`)
      setSecrets(secrets.filter(s => s.id !== id))
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to delete secret')
    }
  }

  const rotateSecret = async (id: number) => {
    try {
      await api.post(`/secrets/${id}/rotate`)
      fetchSecrets()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to rotate secret')
    }
  }

  const revealSecret = async (id: number) => {
    if (revealedSecrets.has(id)) {
      setRevealedSecrets(prev => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
      return
    }

    try {
      const response = await api.get(`/secrets/${id}`)
      setSecretValues(prev => ({ ...prev, [id]: response.data.value }))
      setRevealedSecrets(prev => new Set(prev).add(id))

      // Auto-hide after 30 seconds
      setTimeout(() => {
        setRevealedSecrets(prev => {
          const next = new Set(prev)
          next.delete(id)
          return next
        })
      }, 30000)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to reveal secret')
    }
  }

  const copyToClipboard = async (id: number) => {
    const value = secretValues[id]
    if (!value) return

    try {
      await navigator.clipboard.writeText(value)
      setCopied(id)
      setTimeout(() => setCopied(null), 2000)
    } catch (err) {
      setError('Failed to copy to clipboard')
    }
  }

  const handleSelect = (secret: Secret) => {
    const value = secretValues[secret.id]
    if (value && onSelect) {
      onSelect(secret.name, value)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-48">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-500"></div>
      </div>
    )
  }

  return (
    <div className="bg-gray-900/50 rounded-lg border border-gray-700 p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-white flex items-center gap-2">
          <Shield className="w-5 h-5 text-cyan-500" />
          Secrets Manager
        </h3>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="flex items-center gap-2 px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md text-sm transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add Secret
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-500/20 border border-red-500/50 rounded-lg flex items-center gap-2 text-red-400">
          <AlertCircle className="w-4 h-4" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto text-red-300 hover:text-red-100">
            &times;
          </button>
        </div>
      )}

      {showCreateForm && (
        <form onSubmit={createSecret} className="mb-4 p-4 bg-gray-800/50 rounded-lg border border-gray-700">
          <div className="grid gap-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">Name</label>
                <input
                  type="text"
                  value={newSecret.name}
                  onChange={e => setNewSecret(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="e.g., OPENAI_API_KEY"
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
                  required
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Type</label>
                <select
                  value={newSecret.type}
                  onChange={e => setNewSecret(prev => ({ ...prev, type: e.target.value as Secret['type'] }))}
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
                >
                  <option value="environment">Environment Variable</option>
                  <option value="api_key">API Key</option>
                  <option value="database">Database Credential</option>
                  <option value="oauth">OAuth Token</option>
                  <option value="ssh">SSH Key</option>
                  <option value="generic">Generic Secret</option>
                </select>
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Value</label>
              <input
                type="password"
                value={newSecret.value}
                onChange={e => setNewSecret(prev => ({ ...prev, value: e.target.value }))}
                placeholder="Enter secret value"
                className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none font-mono"
                required
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Description (optional)</label>
              <input
                type="text"
                value={newSecret.description}
                onChange={e => setNewSecret(prev => ({ ...prev, description: e.target.value }))}
                placeholder="What is this secret used for?"
                className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setShowCreateForm(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-md text-sm transition-colors"
              >
                Create Secret
              </button>
            </div>
          </div>
        </form>
      )}

      <div className="space-y-2">
        {secrets.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <Lock className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>No secrets configured</p>
            <p className="text-sm">Add API keys, database credentials, and other secrets securely</p>
          </div>
        ) : (
          secrets.map(secret => (
            <div
              key={secret.id}
              className="flex items-center justify-between p-3 bg-gray-800/30 rounded-lg border border-gray-700 hover:border-gray-600 transition-colors group"
            >
              <div className="flex items-center gap-3">
                <div className="p-2 bg-gray-700/50 rounded-lg text-cyan-400">
                  {secretTypeIcons[secret.type]}
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-white font-medium">{secret.name}</span>
                    <span className="text-xs px-2 py-0.5 bg-gray-700 rounded text-gray-400">
                      {secretTypeLabels[secret.type]}
                    </span>
                  </div>
                  {secret.description && (
                    <p className="text-sm text-gray-500">{secret.description}</p>
                  )}
                  {revealedSecrets.has(secret.id) && secretValues[secret.id] && (
                    <div className="mt-1 flex items-center gap-2">
                      <code className="text-sm text-cyan-400 bg-gray-900 px-2 py-0.5 rounded font-mono">
                        {secretValues[secret.id]}
                      </code>
                      <button
                        onClick={() => copyToClipboard(secret.id)}
                        className="p-1 hover:bg-gray-700 rounded"
                        title="Copy to clipboard"
                      >
                        {copied === secret.id ? (
                          <Check className="w-4 h-4 text-green-400" />
                        ) : (
                          <Copy className="w-4 h-4 text-gray-400" />
                        )}
                      </button>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button
                  onClick={() => revealSecret(secret.id)}
                  className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-white transition-colors"
                  title={revealedSecrets.has(secret.id) ? 'Hide' : 'Reveal'}
                >
                  {revealedSecrets.has(secret.id) ? (
                    <EyeOff className="w-4 h-4" />
                  ) : (
                    <Eye className="w-4 h-4" />
                  )}
                </button>
                <button
                  onClick={() => rotateSecret(secret.id)}
                  className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-yellow-400 transition-colors"
                  title="Rotate encryption"
                >
                  <RefreshCw className="w-4 h-4" />
                </button>
                {onSelect && revealedSecrets.has(secret.id) && (
                  <button
                    onClick={() => handleSelect(secret)}
                    className="px-2 py-1 bg-cyan-600 hover:bg-cyan-500 rounded-md text-xs text-white transition-colors"
                  >
                    Use
                  </button>
                )}
                <button
                  onClick={() => deleteSecret(secret.id)}
                  className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-red-400 transition-colors"
                  title="Delete"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      <div className="mt-4 pt-4 border-t border-gray-700">
        <p className="text-xs text-gray-500 flex items-center gap-1">
          <Shield className="w-3 h-3" />
          Secrets are encrypted with AES-256. Values are never stored in plain text.
        </p>
      </div>
    </div>
  )
}
