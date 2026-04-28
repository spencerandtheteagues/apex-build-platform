import React, { useState, useEffect, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import apiService from '@/services/api'
import {
  Button,
  Badge,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
  LoadingOverlay,
} from '@/components/ui'
import {
  Building,
  Users,
  Shield,
  Key,
  Activity,
  ChevronRight,
  UserPlus,
  Settings,
  Lock,
  Globe,
  Mail,
  MoreVertical,
  CheckCircle2,
  AlertCircle,
  Save,
  RefreshCw,
  Eye,
  EyeOff,
} from 'lucide-react'
import type { Organization, OrganizationMember, Role, AuditLog, SSOConfigRequest } from '@/types'

type TabId = 'general' | 'members' | 'rbac' | 'sso' | 'audit'

export const OrganizationSettings = () => {
  const [activeTab, setActiveTab] = useState<TabId>('general')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [selectedOrg, setSelectedOrg] = useState<Organization | null>(null)
  const [roles, setRoles] = useState<Role[]>([])
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([])
  const [auditTotal, setAuditTotal] = useState(0)
  const [auditPage, setAuditPage] = useState(1)
  const [auditPageSize] = useState(50)

  const [ssoConfig, setSsoConfig] = useState<SSOConfigRequest>({
    saml_entity_id: '',
    saml_sso_url: '',
    saml_certificate: '',
    scim_enabled: false,
  })
  const [savingSSO, setSavingSSO] = useState(false)
  const [showCert, setShowCert] = useState(false)

  const { user } = useStore()

  const showToast = useCallback((msg: string, type: 'success' | 'error' = 'success') => {
    if (type === 'success') {
      setSuccess(msg)
      setError(null)
    } else {
      setError(msg)
      setSuccess(null)
    }
    setTimeout(() => {
      setSuccess(null)
      setError(null)
    }, 5000)
  }, [])

  // Load organizations on mount
  useEffect(() => {
    const loadOrgs = async () => {
      setLoading(true)
      try {
        const res = await apiService.getOrganizations()
        if (res.success && res.organizations) {
          setOrganizations(res.organizations)
          if (res.organizations.length > 0) {
            setSelectedOrg(res.organizations[0])
          }
        } else {
          setError(res.error || 'Failed to load organizations')
        }
      } catch (err: any) {
        setError(err.message || 'Failed to load organizations')
      } finally {
        setLoading(false)
      }
    }
    loadOrgs()
  }, [])

  // Load org details when selected
  useEffect(() => {
    if (!selectedOrg) return
    const loadOrg = async () => {
      setLoading(true)
      try {
        const res = await apiService.getOrganization(selectedOrg.id)
        if (res.success && res.organization) {
          setSelectedOrg(res.organization)
          // Pre-fill SSO config if org already has it
          if (res.organization.saml_entity_id || res.organization.saml_sso_url) {
            setSsoConfig({
              saml_entity_id: res.organization.saml_entity_id || '',
              saml_sso_url: res.organization.saml_sso_url || '',
              saml_certificate: res.organization.saml_certificate || '',
              scim_enabled: res.organization.scim_enabled,
            })
          }
        } else {
          setError(res.error || 'Failed to load organization')
        }
      } catch (err: any) {
        setError(err.message || 'Failed to load organization')
      } finally {
        setLoading(false)
      }
    }
    loadOrg()
  }, [selectedOrg?.id])

  // Load tab-specific data
  useEffect(() => {
    if (!selectedOrg) return
    if (activeTab === 'rbac') {
      const loadRoles = async () => {
        setLoading(true)
        try {
          const res = await apiService.getRoles(selectedOrg.id)
          if (res.success && res.roles) {
            setRoles(res.roles)
          }
        } catch (err: any) {
          setError(err.message || 'Failed to load roles')
        } finally {
          setLoading(false)
        }
      }
      loadRoles()
    }
    if (activeTab === 'audit') {
      const loadLogs = async () => {
        setLoading(true)
        try {
          const res = await apiService.getAuditLogs(selectedOrg.id, {
            page: auditPage,
            page_size: auditPageSize,
          })
          if (res.success && res.audit_logs) {
            setAuditLogs(res.audit_logs)
            setAuditTotal(res.total || 0)
          }
        } catch (err: any) {
          setError(err.message || 'Failed to load audit logs')
        } finally {
          setLoading(false)
        }
      }
      loadLogs()
    }
  }, [activeTab, selectedOrg, auditPage, auditPageSize])

  const handleSaveSSO = async () => {
    if (!selectedOrg) return
    if (!ssoConfig.saml_entity_id || !ssoConfig.saml_sso_url) {
      showToast('Entity ID and SSO URL are required', 'error')
      return
    }
    setSavingSSO(true)
    try {
      const res = await apiService.configureSSO(selectedOrg.id, ssoConfig)
      if (res.success) {
        showToast('SSO configuration saved successfully')
        setSelectedOrg(prev => prev ? { ...prev, sso_enabled: true } : prev)
      } else {
        showToast(res.error || 'Failed to save SSO config', 'error')
      }
    } catch (err: any) {
      showToast(err.message || 'Failed to save SSO config', 'error')
    } finally {
      setSavingSSO(false)
    }
  }

  const handleUpdateOrg = async () => {
    if (!selectedOrg) return
    // For now, organization update is not wired to backend
    // The backend only has GET and SSO POST endpoints for orgs
    // Show a message that general settings are read-only until backend supports updates
    showToast('Organization general settings are read-only in this release', 'error')
  }

  const tabs = [
    { id: 'general' as TabId, label: 'General', icon: Building },
    { id: 'members' as TabId, label: 'Members', icon: Users },
    { id: 'rbac' as TabId, label: 'Roles & Permissions', icon: Shield },
    { id: 'sso' as TabId, label: 'SSO / SAML', icon: Key },
    { id: 'audit' as TabId, label: 'Audit Logs', icon: Activity },
  ]

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '—'
    return new Date(dateStr).toLocaleString()
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return 'success'
      case 'pending': return 'warning'
      case 'suspended': return 'error'
      default: return 'secondary'
    }
  }

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical': return 'error'
      case 'error': return 'error'
      case 'warning': return 'warning'
      default: return 'secondary'
    }
  }

  return (
    <div className="h-full flex flex-col bg-black/50 backdrop-blur-xl text-white p-6">
      <div className="max-w-6xl mx-auto w-full">
        {/* Header */}
        <div className="flex items-center gap-4 mb-8">
          <div className="w-12 h-12 bg-gradient-to-br from-red-600 to-red-900 rounded-xl flex items-center justify-center shadow-lg shadow-red-900/20">
            <Building className="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 className="text-3xl font-bold text-white">Enterprise Workspace</h1>
            <p className="text-gray-400">Manage your organization, team, and security settings</p>
          </div>
        </div>

        {/* Alert Messages */}
        {(error || success) && (
          <div className={cn(
            'mb-6 p-4 rounded-lg border flex items-center gap-3',
            error
              ? 'bg-red-950/30 border-red-900/50 text-red-400'
              : 'bg-green-950/30 border-green-900/50 text-green-400'
          )}>
            {error ? <AlertCircle className="w-5 h-5" /> : <CheckCircle2 className="w-5 h-5" />}
            <span className="text-sm">{error || success}</span>
          </div>
        )}

        {/* Org Selector */}
        {organizations.length > 1 && (
          <div className="mb-6">
            <select
              className="bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-sm text-white focus:border-red-600 outline-none"
              value={selectedOrg?.id || ''}
              onChange={(e) => {
                const org = organizations.find(o => o.id === Number(e.target.value))
                if (org) setSelectedOrg(org)
              }}
            >
              {organizations.map(o => (
                <option key={o.id} value={o.id}>{o.name}</option>
              ))}
            </select>
          </div>
        )}

        <div className="flex gap-8">
          {/* Sidebar Tabs */}
          <div className="w-64 shrink-0 space-y-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={cn(
                  'w-full flex items-center gap-3 px-4 py-3 rounded-lg transition-all duration-200',
                  activeTab === tab.id
                    ? 'bg-red-900/20 text-red-400 border border-red-900/50 shadow-sm'
                    : 'text-gray-400 hover:text-white hover:bg-gray-800/50'
                )}
              >
                <tab.icon className="w-4 h-4" />
                <span className="text-sm font-medium">{tab.label}</span>
                {activeTab === tab.id && <ChevronRight className="w-4 h-4 ml-auto" />}
              </button>
            ))}
          </div>

          {/* Tab Content */}
          <div className="flex-1 min-w-0">
            {loading && (
              <div className="flex items-center justify-center h-64 text-gray-500">
                <RefreshCw className="w-8 h-8 animate-spin mr-3" />
                <span>Loading...</span>
              </div>
            )}

            {!loading && !selectedOrg && (
              <div className="flex flex-col items-center justify-center h-64 text-gray-500">
                <Building className="w-12 h-12 mb-4 opacity-20" />
                <p>No organizations found.</p>
                <p className="text-sm mt-2">Enterprise features require a team or higher plan.</p>
              </div>
            )}

            {!loading && selectedOrg && activeTab === 'general' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <CardTitle>Workspace Identity</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm font-medium text-gray-400 mb-1 block">Organization Name</label>
                        <Input value={selectedOrg.name} disabled variant="cyberpunk" />
                      </div>
                      <div>
                        <label className="text-sm font-medium text-gray-400 mb-1 block">Organization Slug</label>
                        <Input value={selectedOrg.slug} disabled variant="cyberpunk" />
                      </div>
                    </div>
                    <div>
                      <label className="text-sm font-medium text-gray-400 mb-1 block">Description</label>
                      <Input value={selectedOrg.description || ''} disabled variant="cyberpunk" />
                    </div>
                    <div>
                      <label className="text-sm font-medium text-gray-400 mb-1 block">Website</label>
                      <Input value={selectedOrg.website || ''} disabled variant="cyberpunk" />
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm font-medium text-gray-400 mb-1 block">Billing Email</label>
                        <Input value={selectedOrg.billing_email || ''} disabled variant="cyberpunk" />
                      </div>
                      <div>
                        <label className="text-sm font-medium text-gray-400 mb-1 block">Subscription Type</label>
                        <Badge variant="success" className="mt-1">{selectedOrg.subscription_type}</Badge>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-sm font-medium text-gray-400 mb-1 block">Max Members</label>
                        <Input value={String(selectedOrg.max_members)} disabled variant="cyberpunk" />
                      </div>
                      <div>
                        <label className="text-sm font-medium text-gray-400 mb-1 block">Max Projects</label>
                        <Input value={String(selectedOrg.max_projects)} disabled variant="cyberpunk" />
                      </div>
                    </div>
                    <div className="pt-4 border-t border-gray-800">
                      <p className="text-xs text-gray-500">
                        General settings are managed through your billing portal. Contact support to make changes.
                      </p>
                    </div>
                  </CardContent>
                </Card>

                <Card variant="cyberpunk" className="border-red-900/30">
                  <CardHeader>
                    <CardTitle className="text-red-400">Danger Zone</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm text-gray-400 mb-4">
                      Deleting your organization will immediately remove access for all members and delete all projects. This action is irreversible.
                    </p>
                    <Button variant="outline" className="border-red-900/50 text-red-500 hover:bg-red-900/10" disabled>
                      Delete Organization
                    </Button>
                  </CardContent>
                </Card>
              </div>
            )}

            {!loading && selectedOrg && activeTab === 'members' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle>Members</CardTitle>
                      <Badge variant="secondary">{selectedOrg.members?.length || 0} / {selectedOrg.max_members}</Badge>
                    </div>
                  </CardHeader>
                  <CardContent>
                    {!selectedOrg.members || selectedOrg.members.length === 0 ? (
                      <p className="text-gray-500 text-sm">No members yet.</p>
                    ) : (
                      <div className="space-y-2">
                        {selectedOrg.members.map((member) => (
                          <div key={member.id} className="flex items-center gap-4 p-3 bg-gray-900/50 rounded-lg border border-gray-800">
                            <div className="w-8 h-8 rounded-full bg-red-600/20 flex items-center justify-center text-red-400 font-bold text-sm">
                              {(member.user?.username?.charAt(0) || 'U').toUpperCase()}
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="text-sm font-medium text-white">{member.user?.username || 'Unknown'}</div>
                              <div className="text-xs text-gray-500">{member.user?.email || member.saml_name_id || 'No email'}</div>
                            </div>
                            <Badge variant={getStatusColor(member.status)}>{member.status}</Badge>
                            <span className="text-xs text-gray-400">{member.role?.name || 'Member'}</span>
                            {member.provisioned_by !== 'manual' && (
                              <Badge variant="outline" className="text-xs">{member.provisioned_by}</Badge>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}

            {!loading && selectedOrg && activeTab === 'rbac' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <CardTitle>Roles & Permissions</CardTitle>
                  </CardHeader>
                  <CardContent>
                    {!selectedOrg.advanced_rbac_enabled ? (
                      <div className="p-4 bg-yellow-950/20 border border-yellow-900/30 rounded-lg">
                        <p className="text-sm text-yellow-400">Advanced RBAC is not enabled for this organization.</p>
                        <p className="text-xs text-gray-500 mt-1">Upgrade to Enterprise plan to enable custom roles and granular permissions.</p>
                      </div>
                    ) : roles.length === 0 ? (
                      <p className="text-gray-500 text-sm">No custom roles defined.</p>
                    ) : (
                      <div className="space-y-4">
                        {roles.map((role) => (
                          <div key={role.id} className="p-4 bg-gray-900/50 rounded-lg border border-gray-800">
                            <div className="flex items-center gap-2 mb-2">
                              <span className="font-semibold text-white">{role.name}</span>
                              {role.is_system && <Badge variant="outline" className="text-xs">System</Badge>}
                              {role.is_default && <Badge variant="secondary" className="text-xs">Default</Badge>}
                            </div>
                            <p className="text-sm text-gray-400 mb-3">{role.description || 'No description'}</p>
                            {role.permissions && role.permissions.length > 0 && (
                              <div className="flex flex-wrap gap-2">
                                {role.permissions.map((perm) => (
                                  <Badge key={perm.id} variant="outline" className="text-xs">
                                    {perm.resource}:{perm.action}
                                  </Badge>
                                ))}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}

            {!loading && selectedOrg && activeTab === 'sso' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle>SAML 2.0 Configuration</CardTitle>
                      <Badge variant={selectedOrg.sso_enabled ? 'success' : 'secondary'}>
                        {selectedOrg.sso_enabled ? 'Active' : 'Inactive'}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <p className="text-sm text-gray-400">
                      Connect your identity provider (Okta, Azure AD, Google) to enable single sign-on for your team.
                    </p>
                    <Input
                      label="Entity ID"
                      value={ssoConfig.saml_entity_id}
                      onChange={(e) => setSsoConfig(prev => ({ ...prev, saml_entity_id: e.target.value }))}
                      variant="cyberpunk"
                      placeholder="apex-build-your-org"
                    />
                    <Input
                      label="SSO URL"
                      value={ssoConfig.saml_sso_url}
                      onChange={(e) => setSsoConfig(prev => ({ ...prev, saml_sso_url: e.target.value }))}
                      variant="cyberpunk"
                      placeholder="https://your-idp.com/sso/saml2"
                    />
                    <div>
                      <label className="text-sm font-medium text-gray-400 mb-2 block">Public Certificate (x509)</label>
                      <div className="relative">
                        <textarea
                          className="w-full h-32 bg-gray-900 border border-gray-700 rounded-lg p-3 text-xs font-mono text-gray-300 focus:border-red-600 outline-none resize-none"
                          value={ssoConfig.saml_certificate}
                          onChange={(e) => setSsoConfig(prev => ({ ...prev, saml_certificate: e.target.value }))}
                          placeholder="-----BEGIN CERTIFICATE-----\n..."
                        />
                        <button
                          onClick={() => setShowCert(!showCert)}
                          className="absolute top-2 right-2 text-gray-500 hover:text-white"
                        >
                          {showCert ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </button>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="scim-enabled"
                        checked={ssoConfig.scim_enabled}
                        onChange={(e) => setSsoConfig(prev => ({ ...prev, scim_enabled: e.target.checked }))}
                        className="w-4 h-4 rounded border-gray-700 bg-gray-900 text-red-600 focus:ring-red-600"
                      />
                      <label htmlFor="scim-enabled" className="text-sm text-gray-300">Enable SCIM User Provisioning</label>
                    </div>
                    <div className="pt-4 border-t border-gray-800 flex items-center gap-3">
                      <Button
                        variant="primary"
                        className="bg-red-600 hover:bg-red-500"
                        onClick={handleSaveSSO}
                        disabled={savingSSO}
                      >
                        {savingSSO ? (
                          <>
                            <RefreshCw className="w-4 h-4 animate-spin mr-2" />
                            Saving...
                          </>
                        ) : (
                          <>
                            <Save className="w-4 h-4 mr-2" />
                            Save SSO Settings
                          </>
                        )}
                      </Button>
                      {selectedOrg.sso_enabled && (
                        <div className="flex items-center gap-2 text-sm text-green-400">
                          <CheckCircle2 className="w-4 h-4" />
                          <span>SSO is configured</span>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </div>
            )}

            {!loading && selectedOrg && activeTab === 'audit' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle>Audit Logs</CardTitle>
                      <Badge variant="secondary">{auditTotal} total</Badge>
                    </div>
                  </CardHeader>
                  <CardContent>
                    {!selectedOrg.audit_logs_enabled ? (
                      <div className="p-4 bg-yellow-950/20 border border-yellow-900/30 rounded-lg">
                        <p className="text-sm text-yellow-400">Audit logs are not enabled for this organization.</p>
                      </div>
                    ) : auditLogs.length === 0 ? (
                      <p className="text-gray-500 text-sm">No audit logs found.</p>
                    ) : (
                      <div className="space-y-2">
                        {auditLogs.map((log) => (
                          <div key={log.id} className="p-3 bg-gray-900/50 rounded-lg border border-gray-800 text-sm">
                            <div className="flex items-center gap-2 mb-1">
                              <Badge variant={getSeverityColor(log.severity)} className="text-xs">{log.severity}</Badge>
                              <span className="text-gray-300 font-medium">{log.action}</span>
                              <span className="text-gray-500">on {log.resource_type}</span>
                              <span className="text-gray-500 ml-auto">{formatDate(log.created_at)}</span>
                            </div>
                            <div className="text-gray-400">{log.description || 'No description'}</div>
                            <div className="flex items-center gap-2 mt-1 text-xs text-gray-500">
                              <span>User: {log.username || 'System'}</span>
                              <span>Outcome: {log.outcome}</span>
                            </div>
                          </div>
                        ))}
                        {auditTotal > auditPageSize && (
                          <div className="flex items-center justify-between pt-4">
                            <Button
                              variant="outline"
                              onClick={() => setAuditPage(p => Math.max(1, p - 1))}
                              disabled={auditPage <= 1}
                              className="text-xs"
                            >
                              Previous
                            </Button>
                            <span className="text-sm text-gray-400">
                              Page {auditPage} of {Math.ceil(auditTotal / auditPageSize)}
                            </span>
                            <Button
                              variant="outline"
                              onClick={() => setAuditPage(p => p + 1)}
                              disabled={auditPage >= Math.ceil(auditTotal / auditPageSize)}
                              className="text-xs"
                            >
                              Next
                            </Button>
                          </div>
                        )}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default OrganizationSettings
