// APEX.BUILD Admin Dashboard
// Enterprise administration interface with dark demon theme

import React, { useState, useEffect, useCallback } from 'react'
import { cn, formatRelativeTime, formatFileSize } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import {
  Button,
  Badge,
  Card,
  Loading,
  Input
} from '@/components/ui'
import {
  Users,
  DollarSign,
  Database,
  Shield,
  Ban,
  CheckCircle,
  CreditCard,
  Zap,
  Crown,
  Search,
  RefreshCw,
  Edit3,
  Trash2,
  Eye,
  AlertTriangle
} from 'lucide-react'

interface AdminStats {
  users: {
    total: number
    active: number
    admins: number
    pro: number
  }
  projects: {
    total: number
    active: number
  }
  ai: {
    total_requests: number
    total_cost: number
  }
}

interface SystemStats {
  ai_providers: {
    claude: number
    gpt4: number
    gemini: number
  }
  subscriptions: {
    free: number
    pro: number
    team: number
    enterprise: number
    owner: number
  }
  storage: {
    total_files: number
    total_bytes: number
  }
  timestamp: string
}

interface AIRequest {
  id: number
  provider: string
  model?: string
  cost?: number
  total_tokens?: number
  created_at: string
}

interface User {
  id: number
  username: string
  email: string
  full_name: string
  is_active: boolean
  is_verified: boolean
  is_admin: boolean
  is_super_admin: boolean
  has_unlimited_credits: boolean
  bypass_billing: boolean
  bypass_rate_limits?: boolean
  subscription_type: string
  credit_balance: number
  created_at: string
}

export interface AdminDashboardProps {
  className?: string
}

export const AdminDashboard: React.FC<AdminDashboardProps> = ({ className }) => {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [systemStats, setSystemStats] = useState<SystemStats | null>(null)
  const [recentUsers, setRecentUsers] = useState<User[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [usersLoading, setUsersLoading] = useState(false)
  const [statsError, setStatsError] = useState<string | null>(null)
  const [usersError, setUsersError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [showUserModal, setShowUserModal] = useState(false)
  const [showCreditsModal, setShowCreditsModal] = useState(false)
  const [creditsAmount, setCreditsAmount] = useState('100')
  const [creditsReason, setCreditsReason] = useState('Admin credit adjustment')
  const [detailsUser, setDetailsUser] = useState<User | null>(null)
  const [detailsRequests, setDetailsRequests] = useState<AIRequest[]>([])
  const [showDetailsModal, setShowDetailsModal] = useState(false)

  const { apiService, user, addNotification } = useStore()

  // Check if user is admin
  const isAdmin = user?.is_admin || user?.is_super_admin
  const isSuperAdmin = user?.is_super_admin

  // Fetch admin dashboard data
  const fetchDashboard = useCallback(async () => {
    if (!isAdmin) return

    try {
      setLoading(true)
      setStatsError(null)
      const response = await apiService.client.get('/admin/dashboard')
      setStats(response.data.stats)
      setRecentUsers(response.data.recent_users || [])
    } catch (error) {
      console.error('Failed to fetch admin dashboard:', error)
      setStatsError('Failed to load dashboard stats')
    } finally {
      setLoading(false)
    }
  }, [apiService, isAdmin])

  // Fetch users
  const fetchUsers = useCallback(async () => {
    if (!isAdmin) return

    try {
      setUsersLoading(true)
      setUsersError(null)
      const response = await apiService.client.get('/admin/users', {
        params: { search: searchQuery, limit: 50, page }
      })
      setUsers(response.data.users || [])
      const pagination = response.data.pagination
      if (pagination?.total_pages) {
        setTotalPages(pagination.total_pages)
      }
    } catch (error) {
      console.error('Failed to fetch users:', error)
      setUsersError('Failed to load users')
    } finally {
      setUsersLoading(false)
    }
  }, [apiService, isAdmin, searchQuery, page])

  const fetchSystemStats = useCallback(async () => {
    if (!isAdmin) return

    try {
      const response = await apiService.client.get('/admin/stats')
      setSystemStats(response.data)
    } catch (error) {
      console.error('Failed to fetch system stats:', error)
    }
  }, [apiService, isAdmin])

  useEffect(() => {
    fetchDashboard()
    fetchUsers()
    fetchSystemStats()
  }, [fetchDashboard, fetchUsers, fetchSystemStats])

  useEffect(() => {
    setPage(1)
  }, [searchQuery])

  // Update user
  const updateUser = async (userId: number, updates: Partial<User>) => {
    try {
      await apiService.client.put(`/admin/users/${userId}`, updates)
      fetchUsers()
      setShowUserModal(false)
      addNotification({
        type: 'success',
        title: 'User Updated',
        message: 'User settings saved successfully.',
      })
    } catch (error) {
      console.error('Failed to update user:', error)
      addNotification({
        type: 'error',
        title: 'Update Failed',
        message: 'Unable to update user.',
      })
    }
  }

  // Add credits
  const addCredits = async (userId: number, amount: number) => {
    if (!Number.isFinite(amount) || amount <= 0) {
      addNotification({
        type: 'error',
        title: 'Invalid Amount',
        message: 'Enter a credit amount greater than $0.',
      })
      return
    }
    try {
      await apiService.client.post(`/admin/users/${userId}/credits`, {
        amount,
        reason: creditsReason || 'Admin credit adjustment'
      })
      fetchUsers()
      setShowCreditsModal(false)
      addNotification({
        type: 'success',
        title: 'Credits Added',
        message: `Added $${amount.toFixed(2)} credits.`,
      })
    } catch (error) {
      console.error('Failed to add credits:', error)
      addNotification({
        type: 'error',
        title: 'Credit Update Failed',
        message: 'Unable to add credits.',
      })
    }
  }

  const fetchUserDetails = async (userId: number) => {
    try {
      const response = await apiService.client.get(`/admin/users/${userId}`)
      setDetailsUser(response.data.user)
      setDetailsRequests(response.data.ai_requests || [])
      setShowDetailsModal(true)
    } catch (error) {
      console.error('Failed to fetch user details:', error)
      addNotification({
        type: 'error',
        title: 'Load Failed',
        message: 'Unable to load user details.',
      })
    }
  }

  const deleteUser = async (userId: number) => {
    const confirmed = window.confirm('Delete this user? This cannot be undone.')
    if (!confirmed) return
    try {
      await apiService.client.delete(`/admin/users/${userId}`)
      fetchUsers()
      addNotification({
        type: 'success',
        title: 'User Deleted',
        message: 'User was removed successfully.',
      })
    } catch (error) {
      console.error('Failed to delete user:', error)
      addNotification({
        type: 'error',
        title: 'Delete Failed',
        message: 'Unable to delete user.',
      })
    }
  }

  if (!isAdmin) {
    return (
      <div className="flex items-center justify-center h-screen bg-black">
        <Card variant="cyberpunk" padding="lg" className="text-center">
          <Shield className="w-16 h-16 text-red-500 mx-auto mb-4" />
          <h2 className="text-2xl font-bold text-white mb-2">Access Denied</h2>
          <p className="text-gray-400">You must be an admin to access this page.</p>
        </Card>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-black">
        <Loading size="lg" variant="spinner" label="Loading Admin Dashboard..." />
      </div>
    )
  }

  return (
    <div className={cn('min-h-screen bg-black p-6', className)}>
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-4 mb-4">
          <Shield className="w-10 h-10 text-red-500" />
          <div>
            <h1 className="text-3xl font-bold text-white neon-text-red">
              Admin Dashboard
            </h1>
            <p className="text-gray-400">APEX.BUILD System Administration</p>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => { fetchDashboard(); fetchUsers(); fetchSystemStats(); }}
            className="ml-auto"
          >
            <RefreshCw className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {/* Total Users */}
        <Card variant="cyberpunk" padding="md" className="border-red-900/50">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-400">Total Users</p>
              <p className="text-3xl font-bold text-white">{stats?.users.total || 0}</p>
              <p className="text-xs text-green-500">
                {stats?.users.active || 0} active
              </p>
            </div>
            <Users className="w-10 h-10 text-red-500 opacity-50" />
          </div>
        </Card>

        {/* Admin Users */}
        <Card variant="cyberpunk" padding="md" className="border-purple-900/50">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-400">Admin Users</p>
              <p className="text-3xl font-bold text-white">{stats?.users.admins || 0}</p>
              <p className="text-xs text-purple-400">
                {stats?.users.pro || 0} pro subscribers
              </p>
            </div>
            <Crown className="w-10 h-10 text-purple-500 opacity-50" />
          </div>
        </Card>

        {/* Projects */}
        <Card variant="cyberpunk" padding="md" className="border-cyan-900/50">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-400">Projects</p>
              <p className="text-3xl font-bold text-white">{stats?.projects.total || 0}</p>
              <p className="text-xs text-cyan-400">
                {stats?.projects.active || 0} active
              </p>
            </div>
            <Database className="w-10 h-10 text-cyan-500 opacity-50" />
          </div>
        </Card>

        {/* AI Usage */}
        <Card variant="cyberpunk" padding="md" className="border-orange-900/50">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-400">AI Requests</p>
              <p className="text-3xl font-bold text-white">{stats?.ai.total_requests || 0}</p>
              <p className="text-xs text-orange-400">
                ${(stats?.ai.total_cost || 0).toFixed(2)} total cost
              </p>
            </div>
            <Zap className="w-10 h-10 text-orange-500 opacity-50" />
          </div>
        </Card>
      </div>

      {statsError && (
        <Card variant="cyberpunk" padding="md" className="mb-8 border-red-900/50">
          <div className="flex items-center gap-3 text-red-400">
            <AlertTriangle className="w-5 h-5" />
            <span>{statsError}</span>
          </div>
        </Card>
      )}

      {/* System Stats */}
      <Card variant="cyberpunk" padding="lg" className="mb-8">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-bold text-white">System Stats</h2>
          {systemStats?.timestamp && (
            <span className="text-xs text-gray-400">
              Updated {formatRelativeTime(systemStats.timestamp)}
            </span>
          )}
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <Card variant="cyberpunk" padding="md" className="border-gray-800">
            <h3 className="text-sm text-gray-400 mb-3">AI Provider Usage</h3>
            <div className="space-y-2 text-sm text-gray-200">
              <div className="flex items-center justify-between">
                <span>Claude</span>
                <span className="text-orange-400">{systemStats?.ai_providers.claude || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>GPT-4</span>
                <span className="text-green-400">{systemStats?.ai_providers.gpt4 || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>Gemini</span>
                <span className="text-blue-400">{systemStats?.ai_providers.gemini || 0}</span>
              </div>
            </div>
          </Card>

          <Card variant="cyberpunk" padding="md" className="border-gray-800">
            <h3 className="text-sm text-gray-400 mb-3">Subscriptions</h3>
            <div className="space-y-2 text-sm text-gray-200">
              <div className="flex items-center justify-between">
                <span>Free</span>
                <span>{systemStats?.subscriptions.free || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>Pro</span>
                <span>{systemStats?.subscriptions.pro || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>Team</span>
                <span>{systemStats?.subscriptions.team || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>Enterprise</span>
                <span>{systemStats?.subscriptions.enterprise || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>Owner</span>
                <span>{systemStats?.subscriptions.owner || 0}</span>
              </div>
            </div>
          </Card>

          <Card variant="cyberpunk" padding="md" className="border-gray-800">
            <h3 className="text-sm text-gray-400 mb-3">Storage</h3>
            <div className="space-y-2 text-sm text-gray-200">
              <div className="flex items-center justify-between">
                <span>Total Files</span>
                <span>{systemStats?.storage.total_files || 0}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>Total Bytes</span>
                <span>{formatFileSize(systemStats?.storage.total_bytes || 0)}</span>
              </div>
            </div>
          </Card>
        </div>
      </Card>

      {/* User Management */}
      <Card variant="cyberpunk" padding="lg" className="mb-8">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-bold text-white">User Management</h2>
          <div className="flex items-center gap-4">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <Input
                type="text"
                placeholder="Search users..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10 bg-gray-900 border-gray-700 w-64"
              />
            </div>
          </div>
        </div>

        {usersError && (
          <div className="mb-4 text-sm text-red-400 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4" />
            {usersError}
          </div>
        )}

        {/* Users Table */}
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-800">
                <th className="text-left py-3 px-4 text-gray-400 font-medium">User</th>
                <th className="text-left py-3 px-4 text-gray-400 font-medium">Status</th>
                <th className="text-left py-3 px-4 text-gray-400 font-medium">Subscription</th>
                <th className="text-left py-3 px-4 text-gray-400 font-medium">Credits</th>
                <th className="text-left py-3 px-4 text-gray-400 font-medium">Privileges</th>
                <th className="text-left py-3 px-4 text-gray-400 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {usersLoading ? (
                <tr>
                  <td colSpan={6} className="py-6 text-center text-gray-500">
                    Loading users...
                  </td>
                </tr>
              ) : users.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-6 text-center text-gray-500">
                    No users found.
                  </td>
                </tr>
              ) : users.map((u) => (
                <tr key={u.id} className="border-b border-gray-800/50 hover:bg-gray-900/50">
                  <td className="py-3 px-4">
                    <div>
                      <p className="text-white font-medium">{u.username}</p>
                      <p className="text-gray-400 text-sm">{u.email}</p>
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-2">
                      {u.is_active ? (
                        <Badge variant="success" size="sm">Active</Badge>
                      ) : (
                        <Badge variant="error" size="sm">Inactive</Badge>
                      )}
                      {u.is_verified && (
                        <CheckCircle className="w-4 h-4 text-green-500" />
                      )}
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <Badge
                      variant={
                        u.subscription_type === 'owner' ? 'primary' :
                        u.subscription_type === 'enterprise' ? 'success' :
                        u.subscription_type === 'pro' ? 'info' :
                        'default'
                      }
                      size="sm"
                    >
                      {u.subscription_type}
                    </Badge>
                  </td>
                  <td className="py-3 px-4">
                    {u.has_unlimited_credits ? (
                      <span className="text-yellow-500 font-medium">∞ Unlimited</span>
                    ) : (
                      <span className="text-gray-300">${u.credit_balance.toFixed(2)}</span>
                    )}
                  </td>
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-1">
                      {u.is_super_admin && (
                        <Badge variant="error" size="sm" className="bg-red-900/50">
                          Super Admin
                        </Badge>
                      )}
                      {u.is_admin && !u.is_super_admin && (
                        <Badge variant="warning" size="sm">Admin</Badge>
                      )}
                      {u.bypass_billing && (
                        <CreditCard className="w-4 h-4 text-green-500" aria-label="Bypass Billing" />
                      )}
                      {u.bypass_rate_limits && (
                        <Zap className="w-4 h-4 text-yellow-500" aria-label="Bypass Rate Limits" />
                      )}
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => {
                          setSelectedUser(u)
                          setShowUserModal(true)
                        }}
                        title="Edit user"
                      >
                        <Edit3 className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => {
                          setSelectedUser(u)
                          setCreditsAmount('100')
                          setShowCreditsModal(true)
                        }}
                        title="Add credits"
                      >
                        <DollarSign className="w-4 h-4 text-green-500" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => updateUser(u.id, { is_active: !u.is_active })}
                        title={u.is_active ? 'Deactivate user' : 'Activate user'}
                      >
                        {u.is_active ? (
                          <Ban className="w-4 h-4 text-red-500" />
                        ) : (
                          <CheckCircle className="w-4 h-4 text-green-500" />
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => fetchUserDetails(u.id)}
                        title="View details"
                      >
                        <Eye className="w-4 h-4 text-blue-400" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => deleteUser(u.id)}
                        title="Delete user"
                      >
                        <Trash2 className="w-4 h-4 text-red-500" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        <div className="flex items-center justify-between mt-4 text-sm text-gray-400">
          <span>Page {page} of {totalPages}</span>
          <div className="flex gap-2">
            <Button
              variant="ghost"
              size="xs"
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page <= 1}
            >
              Prev
            </Button>
            <Button
              variant="ghost"
              size="xs"
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
            >
              Next
            </Button>
          </div>
        </div>
      </Card>

      {/* Recent Users */}
      {recentUsers.length > 0 && (
        <Card variant="cyberpunk" padding="lg" className="mb-8">
          <h2 className="text-xl font-bold text-white mb-4">Recent Users</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {recentUsers.map((recent) => (
              <div key={recent.id} className="p-4 bg-gray-900/60 rounded-lg border border-gray-800">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-white font-medium">{recent.username}</p>
                    <p className="text-xs text-gray-500">{recent.email}</p>
                  </div>
                  <Badge variant={recent.is_active ? 'success' : 'error'} size="sm">
                    {recent.is_active ? 'Active' : 'Inactive'}
                  </Badge>
                </div>
                <p className="text-xs text-gray-500 mt-2">
                  Joined {formatRelativeTime(recent.created_at)}
                </p>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* User Edit Modal */}
      {showUserModal && selectedUser && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
          <Card variant="cyberpunk" padding="lg" className="w-full max-w-md">
            <h3 className="text-xl font-bold text-white mb-6">
              Edit User: {selectedUser.username}
            </h3>

            <div className="space-y-4">
              {/* Privileges */}
              <div>
                <label className="text-sm text-gray-400 mb-2 block">Privileges</label>
                <div className="space-y-2">
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.is_admin}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        is_admin: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                      disabled={!isSuperAdmin}
                    />
                    Admin
                  </label>
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.is_super_admin}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        is_super_admin: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                      disabled={!isSuperAdmin}
                    />
                    Super Admin
                  </label>
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.has_unlimited_credits}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        has_unlimited_credits: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                    />
                    Unlimited Credits
                  </label>
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.bypass_billing}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        bypass_billing: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                    />
                    Bypass Billing
                  </label>
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.bypass_rate_limits || false}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        bypass_rate_limits: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                    />
                    Bypass Rate Limits
                  </label>
                </div>
              </div>

              {/* Account Status */}
              <div>
                <label className="text-sm text-gray-400 mb-2 block">Account Status</label>
                <div className="space-y-2">
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.is_active}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        is_active: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                    />
                    Active
                  </label>
                  <label className="flex items-center gap-2 text-white">
                    <input
                      type="checkbox"
                      checked={selectedUser.is_verified}
                      onChange={(e) => setSelectedUser({
                        ...selectedUser,
                        is_verified: e.target.checked
                      })}
                      className="rounded bg-gray-900 border-gray-700"
                    />
                    Verified
                  </label>
                </div>
              </div>

              {/* Subscription */}
              <div>
                <label className="text-sm text-gray-400 mb-2 block">Subscription</label>
                <select
                  value={selectedUser.subscription_type}
                  onChange={(e) => setSelectedUser({
                    ...selectedUser,
                    subscription_type: e.target.value
                  })}
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-white"
                >
                  <option value="free">Free</option>
                  <option value="pro">Pro</option>
                  <option value="team">Team</option>
                  <option value="enterprise">Enterprise</option>
                  <option value="owner">Owner</option>
                </select>
              </div>

              {/* Credit Balance */}
              {!selectedUser.has_unlimited_credits && (
                <div>
                  <label className="text-sm text-gray-400 mb-2 block">Credit Balance</label>
                  <Input
                    type="number"
                    step="0.01"
                    value={String(selectedUser.credit_balance ?? 0)}
                    onChange={(e) => setSelectedUser({
                      ...selectedUser,
                      credit_balance: Number(e.target.value || 0)
                    })}
                    className="bg-gray-900 border-gray-700"
                  />
                </div>
              )}

              {/* Actions */}
              <div className="flex gap-4 pt-4">
                <Button
                  variant="primary"
                  onClick={() => updateUser(selectedUser.id, {
                    is_admin: selectedUser.is_admin,
                    is_super_admin: selectedUser.is_super_admin,
                    has_unlimited_credits: selectedUser.has_unlimited_credits,
                    bypass_billing: selectedUser.bypass_billing,
                    bypass_rate_limits: selectedUser.bypass_rate_limits,
                    subscription_type: selectedUser.subscription_type,
                    is_active: selectedUser.is_active,
                    is_verified: selectedUser.is_verified,
                    credit_balance: selectedUser.credit_balance,
                  })}
                  className="flex-1"
                >
                  Save Changes
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => setShowUserModal(false)}
                >
                  Cancel
                </Button>
              </div>
            </div>
          </Card>
        </div>
      )}

      {/* Credits Modal */}
      {showCreditsModal && selectedUser && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
          <Card variant="cyberpunk" padding="lg" className="w-full max-w-md">
            <h3 className="text-xl font-bold text-white mb-4">
              Add Credits: {selectedUser.username}
            </h3>
            <div className="space-y-4">
              <div>
                <label className="text-sm text-gray-400 mb-2 block">Amount (USD)</label>
                <Input
                  type="number"
                  step="0.01"
                  value={creditsAmount}
                  onChange={(e) => setCreditsAmount(e.target.value)}
                  className="bg-gray-900 border-gray-700"
                />
              </div>
              <div>
                <label className="text-sm text-gray-400 mb-2 block">Reason</label>
                <Input
                  type="text"
                  value={creditsReason}
                  onChange={(e) => setCreditsReason(e.target.value)}
                  className="bg-gray-900 border-gray-700"
                />
              </div>
              <div className="flex gap-4 pt-2">
                <Button
                  variant="primary"
                  className="flex-1"
                  onClick={() => addCredits(selectedUser.id, Number(creditsAmount || 0))}
                >
                  Add Credits
                </Button>
                <Button variant="ghost" onClick={() => setShowCreditsModal(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          </Card>
        </div>
      )}

      {/* User Details Modal */}
      {showDetailsModal && detailsUser && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
          <Card variant="cyberpunk" padding="lg" className="w-full max-w-2xl">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-xl font-bold text-white">
                User Details: {detailsUser.username}
              </h3>
              <Button variant="ghost" size="sm" onClick={() => setShowDetailsModal(false)}>
                Close
              </Button>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm text-gray-300">
              <div>
                <p className="text-gray-500">Email</p>
                <p>{detailsUser.email}</p>
              </div>
              <div>
                <p className="text-gray-500">Joined</p>
                <p>{formatRelativeTime(detailsUser.created_at)}</p>
              </div>
              <div>
                <p className="text-gray-500">Subscription</p>
                <p>{detailsUser.subscription_type}</p>
              </div>
              <div>
                <p className="text-gray-500">Credits</p>
                <p>{detailsUser.has_unlimited_credits ? 'Unlimited' : `$${detailsUser.credit_balance.toFixed(2)}`}</p>
              </div>
            </div>

            <div className="mt-6">
              <h4 className="text-lg font-semibold text-white mb-3">Recent AI Requests</h4>
              {detailsRequests.length === 0 ? (
                <p className="text-sm text-gray-500">No recent AI activity.</p>
              ) : (
                <div className="max-h-64 overflow-y-auto border border-gray-800 rounded-lg">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-gray-800 text-gray-500">
                        <th className="text-left px-3 py-2">Provider</th>
                        <th className="text-left px-3 py-2">Model</th>
                        <th className="text-left px-3 py-2">Tokens</th>
                        <th className="text-left px-3 py-2">Cost</th>
                        <th className="text-left px-3 py-2">Time</th>
                      </tr>
                    </thead>
                    <tbody>
                      {detailsRequests.map((req) => (
                        <tr key={req.id} className="border-b border-gray-800/50">
                          <td className="px-3 py-2">{req.provider}</td>
                          <td className="px-3 py-2">{req.model || 'default'}</td>
                          <td className="px-3 py-2">{req.total_tokens || 0}</td>
                          <td className="px-3 py-2">${(req.cost || 0).toFixed(4)}</td>
                          <td className="px-3 py-2">{formatRelativeTime(req.created_at)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </Card>
        </div>
      )}
    </div>
  )
}

export default AdminDashboard
