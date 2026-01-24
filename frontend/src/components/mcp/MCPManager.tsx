import React, { useState, useEffect, useCallback } from 'react'
import { Plug, PlugZap, Plus, Trash2, Play, Pause, RefreshCw, AlertCircle, Wrench, FileText, ExternalLink, Check, X } from 'lucide-react'
import api from '../../services/api'

interface MCPServer {
  id: number
  name: string
  description?: string
  url: string
  auth_type: 'none' | 'bearer' | 'api_key' | 'custom'
  enabled: boolean
  last_status: 'configured' | 'connected' | 'disconnected' | 'error'
  last_error?: string
  last_connected?: string
  connected: boolean
  tools?: MCPTool[]
  resources?: MCPResource[]
}

interface MCPTool {
  name: string
  description?: string
  inputSchema: any
  server_id?: number
  server_name?: string
}

interface MCPResource {
  uri: string
  name: string
  description?: string
  mimeType?: string
}

interface MCPManagerProps {
  projectId?: number
  onToolCall?: (serverName: string, toolName: string, result: any) => void
}

export default function MCPManager({ projectId, onToolCall }: MCPManagerProps) {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [allTools, setAllTools] = useState<MCPTool[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  const [connecting, setConnecting] = useState<number | null>(null)
  const [showToolCall, setShowToolCall] = useState<{ serverId: number; tool: MCPTool } | null>(null)
  const [toolArgs, setToolArgs] = useState<Record<string, string>>({})
  const [toolResult, setToolResult] = useState<any>(null)
  const [executingTool, setExecutingTool] = useState(false)

  const [newServer, setNewServer] = useState({
    name: '',
    description: '',
    url: '',
    auth_type: 'none' as 'none' | 'bearer' | 'api_key' | 'custom',
    auth_header: '',
    credential: '',
  })

  const fetchServers = useCallback(async () => {
    try {
      setLoading(true)
      const params = projectId ? { project_id: projectId } : {}
      const response = await api.get('/mcp/servers', { params })
      setServers(response.data.servers || [])
      setError(null)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch MCP servers')
    } finally {
      setLoading(false)
    }
  }, [projectId])

  const fetchAllTools = useCallback(async () => {
    try {
      const response = await api.get('/mcp/tools')
      setAllTools(response.data.tools || [])
    } catch (err) {
      console.error('Failed to fetch tools:', err)
    }
  }, [])

  useEffect(() => {
    fetchServers()
    fetchAllTools()
  }, [fetchServers, fetchAllTools])

  const addServer = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/mcp/servers', {
        ...newServer,
        project_id: projectId,
      })
      setNewServer({ name: '', description: '', url: '', auth_type: 'none', auth_header: '', credential: '' })
      setShowAddForm(false)
      fetchServers()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to add MCP server')
    }
  }

  const deleteServer = async (id: number) => {
    if (!confirm('Are you sure you want to delete this MCP server?')) return
    try {
      await api.delete(`/mcp/servers/${id}`)
      setServers(servers.filter(s => s.id !== id))
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to delete server')
    }
  }

  const connectServer = async (id: number) => {
    try {
      setConnecting(id)
      const response = await api.post(`/mcp/servers/${id}/connect`)
      fetchServers()
      fetchAllTools()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to connect to server')
    } finally {
      setConnecting(null)
    }
  }

  const disconnectServer = async (id: number) => {
    try {
      await api.post(`/mcp/servers/${id}/disconnect`)
      fetchServers()
      fetchAllTools()
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to disconnect from server')
    }
  }

  const callTool = async () => {
    if (!showToolCall) return
    try {
      setExecutingTool(true)
      const response = await api.post(`/mcp/servers/${showToolCall.serverId}/tools/call`, {
        tool_name: showToolCall.tool.name,
        arguments: toolArgs,
      })
      setToolResult(response.data.result)
      if (onToolCall) {
        const server = servers.find(s => s.id === showToolCall.serverId)
        onToolCall(server?.name || '', showToolCall.tool.name, response.data.result)
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Tool call failed')
    } finally {
      setExecutingTool(false)
    }
  }

  const openToolCall = (serverId: number, tool: MCPTool) => {
    setShowToolCall({ serverId, tool })
    setToolArgs({})
    setToolResult(null)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-48">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-500"></div>
      </div>
    )
  }

  return (
    <div className="bg-gray-900/50 rounded-lg border border-gray-700 p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-white flex items-center gap-2">
          <PlugZap className="w-5 h-5 text-purple-500" />
          MCP Integrations
        </h3>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="flex items-center gap-2 px-3 py-1.5 bg-purple-600 hover:bg-purple-500 text-white rounded-md text-sm transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add Server
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

      {showAddForm && (
        <form onSubmit={addServer} className="mb-4 p-4 bg-gray-800/50 rounded-lg border border-gray-700">
          <div className="grid gap-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">Name</label>
                <input
                  type="text"
                  value={newServer.name}
                  onChange={e => setNewServer(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="e.g., GitHub Copilot MCP"
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none"
                  required
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Auth Type</label>
                <select
                  value={newServer.auth_type}
                  onChange={e => setNewServer(prev => ({ ...prev, auth_type: e.target.value as any }))}
                  className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none"
                >
                  <option value="none">No Authentication</option>
                  <option value="bearer">Bearer Token</option>
                  <option value="api_key">API Key</option>
                  <option value="custom">Custom Header</option>
                </select>
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">WebSocket URL</label>
              <input
                type="url"
                value={newServer.url}
                onChange={e => setNewServer(prev => ({ ...prev, url: e.target.value }))}
                placeholder="wss://example.com/mcp"
                className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none font-mono text-sm"
                required
              />
            </div>
            {newServer.auth_type !== 'none' && (
              <>
                {newServer.auth_type === 'custom' && (
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">Header Name</label>
                    <input
                      type="text"
                      value={newServer.auth_header}
                      onChange={e => setNewServer(prev => ({ ...prev, auth_header: e.target.value }))}
                      placeholder="X-API-Key"
                      className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none"
                    />
                  </div>
                )}
                <div>
                  <label className="block text-sm text-gray-400 mb-1">
                    {newServer.auth_type === 'bearer' ? 'Bearer Token' : 'API Key / Credential'}
                  </label>
                  <input
                    type="password"
                    value={newServer.credential}
                    onChange={e => setNewServer(prev => ({ ...prev, credential: e.target.value }))}
                    placeholder="Enter credential"
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none font-mono"
                  />
                </div>
              </>
            )}
            <div>
              <label className="block text-sm text-gray-400 mb-1">Description (optional)</label>
              <input
                type="text"
                value={newServer.description}
                onChange={e => setNewServer(prev => ({ ...prev, description: e.target.value }))}
                placeholder="What does this MCP server provide?"
                className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none"
              />
            </div>
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setShowAddForm(false)}
                className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-md text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                type="submit"
                className="px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-md text-sm transition-colors"
              >
                Add Server
              </button>
            </div>
          </div>
        </form>
      )}

      {/* Servers List */}
      <div className="space-y-3">
        {servers.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <Plug className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>No MCP servers configured</p>
            <p className="text-sm">Connect to external AI tools and services via MCP</p>
          </div>
        ) : (
          servers.map(server => (
            <div
              key={server.id}
              className="p-4 bg-gray-800/30 rounded-lg border border-gray-700 hover:border-gray-600 transition-colors"
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-3">
                  <div className={`p-2 rounded-lg ${server.connected ? 'bg-green-500/20 text-green-400' : 'bg-gray-700/50 text-gray-400'}`}>
                    {server.connected ? <PlugZap className="w-5 h-5" /> : <Plug className="w-5 h-5" />}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-white font-medium">{server.name}</span>
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        server.last_status === 'connected' ? 'bg-green-500/20 text-green-400' :
                        server.last_status === 'error' ? 'bg-red-500/20 text-red-400' :
                        'bg-gray-700 text-gray-400'
                      }`}>
                        {server.last_status}
                      </span>
                    </div>
                    <p className="text-sm text-gray-500 font-mono">{server.url}</p>
                    {server.last_error && (
                      <p className="text-xs text-red-400 mt-1">{server.last_error}</p>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {server.connected ? (
                    <button
                      onClick={() => disconnectServer(server.id)}
                      className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-yellow-400 transition-colors"
                      title="Disconnect"
                    >
                      <Pause className="w-4 h-4" />
                    </button>
                  ) : (
                    <button
                      onClick={() => connectServer(server.id)}
                      disabled={connecting === server.id}
                      className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-green-400 transition-colors disabled:opacity-50"
                      title="Connect"
                    >
                      {connecting === server.id ? (
                        <RefreshCw className="w-4 h-4 animate-spin" />
                      ) : (
                        <Play className="w-4 h-4" />
                      )}
                    </button>
                  )}
                  <button
                    onClick={() => deleteServer(server.id)}
                    className="p-2 hover:bg-gray-700 rounded-md text-gray-400 hover:text-red-400 transition-colors"
                    title="Delete"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {/* Tools from this server */}
              {server.connected && server.tools && server.tools.length > 0 && (
                <div className="mt-3 pt-3 border-t border-gray-700">
                  <p className="text-xs text-gray-500 mb-2 flex items-center gap-1">
                    <Wrench className="w-3 h-3" />
                    Available Tools ({server.tools.length})
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {server.tools.map(tool => (
                      <button
                        key={tool.name}
                        onClick={() => openToolCall(server.id, tool)}
                        className="px-2 py-1 bg-purple-500/20 hover:bg-purple-500/30 text-purple-300 rounded text-xs transition-colors"
                        title={tool.description}
                      >
                        {tool.name}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {/* Resources from this server */}
              {server.connected && server.resources && server.resources.length > 0 && (
                <div className="mt-3 pt-3 border-t border-gray-700">
                  <p className="text-xs text-gray-500 mb-2 flex items-center gap-1">
                    <FileText className="w-3 h-3" />
                    Available Resources ({server.resources.length})
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {server.resources.map(resource => (
                      <span
                        key={resource.uri}
                        className="px-2 py-1 bg-cyan-500/20 text-cyan-300 rounded text-xs"
                        title={resource.description}
                      >
                        {resource.name}
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {/* All Available Tools Section */}
      {allTools.length > 0 && (
        <div className="mt-6 pt-4 border-t border-gray-700">
          <h4 className="text-sm font-medium text-gray-300 mb-3 flex items-center gap-2">
            <Wrench className="w-4 h-4" />
            All Available Tools ({allTools.length})
          </h4>
          <div className="grid gap-2">
            {allTools.map(tool => (
              <div
                key={`${tool.server_id}-${tool.name}`}
                className="flex items-center justify-between p-2 bg-gray-800/30 rounded-lg text-sm"
              >
                <div>
                  <span className="text-white">{tool.name}</span>
                  {tool.server_name && (
                    <span className="ml-2 text-xs text-gray-500">from {tool.server_name}</span>
                  )}
                  {tool.description && (
                    <p className="text-xs text-gray-500 mt-0.5">{tool.description}</p>
                  )}
                </div>
                {tool.server_id && (
                  <button
                    onClick={() => openToolCall(tool.server_id!, tool)}
                    className="px-2 py-1 bg-purple-600 hover:bg-purple-500 text-white rounded text-xs transition-colors"
                  >
                    Execute
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Tool Call Modal */}
      {showToolCall && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
          <div className="bg-gray-900 rounded-lg border border-gray-700 max-w-lg w-full max-h-[80vh] overflow-auto">
            <div className="p-4 border-b border-gray-700 flex items-center justify-between">
              <h3 className="text-lg font-medium text-white flex items-center gap-2">
                <Wrench className="w-5 h-5 text-purple-500" />
                {showToolCall.tool.name}
              </h3>
              <button
                onClick={() => {
                  setShowToolCall(null)
                  setToolResult(null)
                }}
                className="p-1 hover:bg-gray-800 rounded"
              >
                <X className="w-5 h-5 text-gray-400" />
              </button>
            </div>
            <div className="p-4">
              {showToolCall.tool.description && (
                <p className="text-sm text-gray-400 mb-4">{showToolCall.tool.description}</p>
              )}

              {/* Arguments Form */}
              {showToolCall.tool.inputSchema?.properties && (
                <div className="space-y-3 mb-4">
                  {Object.entries(showToolCall.tool.inputSchema.properties).map(([key, schema]: [string, any]) => (
                    <div key={key}>
                      <label className="block text-sm text-gray-400 mb-1">
                        {key}
                        {showToolCall.tool.inputSchema.required?.includes(key) && (
                          <span className="text-red-400 ml-1">*</span>
                        )}
                      </label>
                      <input
                        type={schema.type === 'number' ? 'number' : 'text'}
                        value={toolArgs[key] || ''}
                        onChange={e => setToolArgs(prev => ({ ...prev, [key]: e.target.value }))}
                        placeholder={schema.description || ''}
                        className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-md text-white focus:border-purple-500 focus:outline-none text-sm"
                      />
                      {schema.description && (
                        <p className="text-xs text-gray-500 mt-1">{schema.description}</p>
                      )}
                    </div>
                  ))}
                </div>
              )}

              <button
                onClick={callTool}
                disabled={executingTool}
                className="w-full py-2 bg-purple-600 hover:bg-purple-500 disabled:bg-purple-800 text-white rounded-md transition-colors flex items-center justify-center gap-2"
              >
                {executingTool ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    Executing...
                  </>
                ) : (
                  <>
                    <Play className="w-4 h-4" />
                    Execute Tool
                  </>
                )}
              </button>

              {/* Result */}
              {toolResult && (
                <div className="mt-4 p-3 bg-gray-800 rounded-lg">
                  <p className="text-xs text-gray-400 mb-2 flex items-center gap-1">
                    {toolResult.isError ? (
                      <X className="w-3 h-3 text-red-400" />
                    ) : (
                      <Check className="w-3 h-3 text-green-400" />
                    )}
                    Result
                  </p>
                  <pre className="text-sm text-white whitespace-pre-wrap overflow-auto max-h-48 font-mono">
                    {toolResult.content?.map((c: any, i: number) => (
                      <div key={i}>{c.text || JSON.stringify(c, null, 2)}</div>
                    ))}
                  </pre>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      <div className="mt-4 pt-4 border-t border-gray-700">
        <p className="text-xs text-gray-500 flex items-center gap-1">
          <ExternalLink className="w-3 h-3" />
          MCP (Model Context Protocol) enables connection to external AI tools and services.
        </p>
      </div>
    </div>
  )
}
