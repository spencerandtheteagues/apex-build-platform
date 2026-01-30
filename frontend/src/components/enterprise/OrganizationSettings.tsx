// APEX.BUILD Organization Settings
// Enterprise organization management, RBAC, and SSO configuration

import React, { useState, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import {
  Button,
  Badge,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
  LoadingOverlay
} from '@/components/ui'
import {
  Building,
  Users,
  Shield,
  Key,
  CreditCard,
  Activity,
  ChevronRight,
  UserPlus,
  Settings,
  Lock,
  Globe,
  Mail,
  MoreVertical,
  CheckCircle2
} from 'lucide-react'

export const OrganizationSettings = () => {
  const [activeTab, setActiveTab] = useState<'general' | 'members' | 'rbac' | 'sso' | 'audit'>('general')
  const [loading, setLoading] = useState(false)
  const { apiService } = useStore()

  // Tab navigation
  const tabs = [
    { id: 'general', label: 'General', icon: Building },
    { id: 'members', label: 'Members', icon: Users },
    { id: 'rbac', label: 'Roles & Permissions', icon: Shield },
    { id: 'sso', label: 'SSO / SAML', icon: Key },
    { id: 'audit', label: 'Audit Logs', icon: Activity },
  ]

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

        <div className="flex gap-8">
          {/* Sidebar Tabs */}
          <div className="w-64 shrink-0 space-y-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as any)}
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
            {activeTab === 'general' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <CardTitle>Workspace Identity</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <Input label="Organization Name" defaultValue="Acme Corp" variant="cyberpunk" />
                      <Input label="Organization Slug" defaultValue="acme-corp" variant="cyberpunk" />
                    </div>
                    <Input label="Website" defaultValue="https://acme.corp" variant="cyberpunk" />
                    <div className="pt-4 border-t border-gray-800">
                      <Button variant="primary" className="bg-red-600 hover:bg-red-500">
                        Save Changes
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                <Card variant="cyberpunk" className="border-red-900/30">
                  <CardHeader>
                    <CardTitle className="text-red-400">Danger Zone</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm text-gray-400 mb-4">
                      Deleting your organization will immediately remove access for all 45 members and delete all 124 projects. This action is irreversible.
                    </p>
                    <Button variant="outline" className="border-red-900/50 text-red-500 hover:bg-red-900/10">
                      Delete Organization
                    </Button>
                  </CardContent>
                </Card>
              </div>
            )}

            {activeTab === 'sso' && (
              <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                <Card variant="cyberpunk" className="border-gray-800">
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle>SAML 2.0 Configuration</CardTitle>
                      <Badge variant="success">Active</Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <p className="text-sm text-gray-400">
                      Connect your identity provider (Okta, Azure AD, Google) to enable single sign-on for your team.
                    </p>
                    <Input label="SSO URL" defaultValue="https://okta.com/sso/saml2/0oa..." variant="cyberpunk" />
                    <Input label="Entity ID" defaultValue="apex-build-acme" variant="cyberpunk" />
                    <div>
                      <label className="text-sm font-medium text-gray-400 mb-2 block">Public Certificate</label>
                      <textarea
                        className="w-full h-32 bg-gray-900 border border-gray-700 rounded-lg p-3 text-xs font-mono text-gray-300 focus:border-red-600 outline-none"
                        defaultValue="-----BEGIN CERTIFICATE-----\nMIIDBTCCAe2gAwIBAgIQN..."
                      />
                    </div>
                    <div className="flex items-center gap-2 text-sm text-green-400">
                      <CheckCircle2 className="w-4 h-4" />
                      <span>SCIM User Provisioning Enabled</span>
                    </div>
                    <div className="pt-4 border-t border-gray-800">
                      <Button variant="primary" className="bg-red-600 hover:bg-red-500">
                        Update SSO Settings
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              </div>
            )}

            {/* Other tabs can be added here with similar beautiful styling */}
            {activeTab !== 'general' && activeTab !== 'sso' && (
              <div className="flex flex-col items-center justify-center h-64 text-gray-500">
                <Settings className="w-12 h-12 mb-4 opacity-20 animate-spin-slow" />
                <p>Advanced {activeTab.toUpperCase()} dashboard components loading...</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default OrganizationSettings
