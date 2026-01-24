// APEX.BUILD Admin Dashboard
// Enterprise administration interface with dark demon theme

import React, { useState, useEffect, useCallback } from 'react'
import { cn } from '@/lib/utils'
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
  Activity,
  DollarSign,
  Database,
  Shield,
  Settings,
  UserPlus,
  Ban,
  CheckCircle,
  XCircle,
  CreditCard,
  Zap,
  Crown,
  Search,
  RefreshCw,
  Edit3,
  Trash2,
  Eye
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
  subscription_type: string
  credit_balance: number
  created_at: string
}

export interface AdminDashboardProps {
  className?: string
}

export const AdminDashboard: React.FC<AdminDashboardProps> = ({ className }) => {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [showUserModal, setShowUserModal] = useState(false)

  const { apiService, user } = useStore()

  // Check if user is admin
  const isAdmin = user?.is_admin || user?.is_super_admin

  // Fetch admin dashboard data
  const fetchDashboard = useCallback(async () => {
    if (!isAdmin) return

    try {
      setLoading(true)
      const response = await apiService.client.get('/api/v1/admin/dashboard')
      setStats(response.data.stats)
    } catch (error) {
      console.error('Failed to fetch admin dashboard:', error)
    } finally {
      setLoading(false)
    }
  }, [apiService, isAdmin])

  // Fetch users
  const fetchUsers = useCallback(async () => {
    if (!isAdmin) return

    try {
      const response = await apiService.client.get('/api/v1/admin/users', {
        params: { search: searchQuery, limit: 50 }
      })
      setUsers(response.data.users || [])
    } catch (error) {
      console.error('Failed to fetch users:', error)
    }
  }, [apiService, isAdmin, searchQuery])

  useEffect(() => {
    fetchDashboard()
    fetchUsers()
  }, [fetchDashboard, fetchUsers])

  // Update user
  const updateUser = async (userId: number, updates: Partial<User>) => {
    try {
      await apiService.client.put(`/api/v1/admin/users/${userId}`, updates)
      fetchUsers()
      setShowUserModal(false)
    } catch (error) {
      console.error('Failed to update user:', error)
    }
  }

  // Add credits
  const addCredits = async (userId: number, amount: number) => {
    try {
      await apiService.client.post(`/api/v1/admin/users/${userId}/credits`, {
        amount,
        reason: 'Admin credit adjustment'
      })
      fetchUsers()
    } catch (error) {
      console.error('Failed to add credits:', error)
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
            onClick={() => { fetchDashboard(); fetchUsers(); }}
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
              {users.map((u) => (
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
                        <CreditCard className="w-4 h-4 text-green-500" title="Bypass Billing" />
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
                      >
                        <Edit3 className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => addCredits(u.id, 100)}
                        title="Add $100 credits"
                      >
                        <DollarSign className="w-4 h-4 text-green-500" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => updateUser(u.id, { is_active: !u.is_active })}
                      >
                        {u.is_active ? (
                          <Ban className="w-4 h-4 text-red-500" />
                        ) : (
                          <CheckCircle className="w-4 h-4 text-green-500" />
                        )}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

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
                    />
                    Admin
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

              {/* Actions */}
              <div className="flex gap-4 pt-4">
                <Button
                  variant="primary"
                  onClick={() => updateUser(selectedUser.id, {
                    is_admin: selectedUser.is_admin,
                    has_unlimited_credits: selectedUser.has_unlimited_credits,
                    bypass_billing: selectedUser.bypass_billing,
                    subscription_type: selectedUser.subscription_type,
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
    </div>
  )
}

export default AdminDashboard
