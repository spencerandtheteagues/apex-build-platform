import React, { useMemo, useState } from 'react'
import { CheckCircle2, KeyRound, Lock, Trash2 } from 'lucide-react'

import { Button } from '@/components/ui'
import { getApiErrorMessage } from '@/lib/errors'
import { cn, formatRelativeTime } from '@/lib/utils'
import type { Notification } from '@/types'
import type { ApiService, MobileCredentialStatus, MobileCredentialType } from '@/services/api'

type Notify = (notification: Omit<Notification, 'id' | 'timestamp'>) => void

interface CredentialField {
  key: string
  label: string
  kind?: 'text' | 'password' | 'textarea'
  placeholder?: string
}

interface CredentialDefinition {
  type: MobileCredentialType
  label: string
  summary: string
  fields: CredentialField[]
}

interface MobileCredentialPanelProps {
  projectId: number
  credentials: MobileCredentialStatus | null
  apiService: Pick<ApiService, 'createProjectMobileCredential' | 'deleteProjectMobileCredential'>
  addNotification: Notify
  onCredentialsChange: (credentials: MobileCredentialStatus) => void
}

const credentialDefinitions: CredentialDefinition[] = [
  {
    type: 'eas_token',
    label: 'EAS token',
    summary: 'Required to queue Android/iOS binaries through the server-side EAS provider.',
    fields: [{ key: 'token', label: 'EAS access token', kind: 'password', placeholder: 'eas_...' }],
  },
  {
    type: 'apple_app_store_connect',
    label: 'Apple App Store Connect API key',
    summary: 'Required for iOS store-readiness and future TestFlight/App Store upload workflows.',
    fields: [
      { key: 'key_id', label: 'Key ID', placeholder: 'ABC123DEFG' },
      { key: 'issuer_id', label: 'Issuer ID', placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx' },
      { key: 'team_id', label: 'Apple team ID', placeholder: 'TEAM123456' },
      { key: 'private_key', label: 'Private key', kind: 'textarea', placeholder: '-----BEGIN PRIVATE KEY-----' },
    ],
  },
  {
    type: 'google_play_service_account',
    label: 'Google Play service account',
    summary: 'Required for Android store-readiness and future Google Play internal/production upload workflows.',
    fields: [{ key: 'service_account_json', label: 'Service account JSON', kind: 'textarea', placeholder: '{"client_email":"...","private_key":"..."}' }],
  },
  {
    type: 'android_signing',
    label: 'Android signing material',
    summary: 'Optional when EAS manages credentials; required for user-managed keystore workflows.',
    fields: [
      { key: 'keystore_base64', label: 'Keystore base64', kind: 'textarea' },
      { key: 'keystore_password', label: 'Keystore password', kind: 'password' },
      { key: 'key_alias', label: 'Key alias' },
      { key: 'key_password', label: 'Key password', kind: 'password' },
    ],
  },
]

const emptyCredentialValues = (): Partial<Record<MobileCredentialType, Record<string, string>>> => ({})

const isCredentialComplete = (definition: CredentialDefinition, values: Record<string, string> | undefined) =>
  definition.fields.every((field) => (values?.[field.key] || '').trim().length > 0)

const fieldValue = (
  valuesByType: Partial<Record<MobileCredentialType, Record<string, string>>>,
  type: MobileCredentialType,
  key: string,
) => valuesByType[type]?.[key] || ''

export const MobileCredentialPanel: React.FC<MobileCredentialPanelProps> = ({
  projectId,
  credentials,
  apiService,
  addNotification,
  onCredentialsChange,
}) => {
  const [expandedType, setExpandedType] = useState<MobileCredentialType | null>(null)
  const [valuesByType, setValuesByType] = useState(emptyCredentialValues)
  const [actionType, setActionType] = useState<string | null>(null)

  const metadataByType = useMemo(() => {
    const entries = new Map<MobileCredentialType, MobileCredentialStatus['metadata'][number]>()
    for (const item of credentials?.metadata || []) {
      entries.set(item.type, item)
    }
    return entries
  }, [credentials])

  const present = new Set(credentials?.present || [])
  const missing = new Set(credentials?.missing || [])
  const required = new Set(credentials?.required || [])

  const updateValue = (type: MobileCredentialType, key: string, value: string) => {
    setValuesByType((current) => ({
      ...current,
      [type]: {
        ...(current[type] || {}),
        [key]: value,
      },
    }))
  }

  const clearValues = (type: MobileCredentialType) => {
    setValuesByType((current) => {
      const next = { ...current }
      delete next[type]
      return next
    })
  }

  const handleSave = async (definition: CredentialDefinition) => {
    const values = valuesByType[definition.type] || {}
    setActionType(`save-${definition.type}`)
    try {
      const next = await apiService.createProjectMobileCredential(projectId, {
        type: definition.type,
        values,
      })
      onCredentialsChange(next)
      clearValues(definition.type)
      setExpandedType(null)
      addNotification({
        type: 'success',
        title: 'Mobile credential stored',
        message: `${definition.label} was encrypted and scoped to this project.`,
      })
    } catch (error) {
      addNotification({
        type: 'error',
        title: 'Credential save failed',
        message: getApiErrorMessage(error, `Unable to store ${definition.label}.`),
      })
    } finally {
      setActionType(null)
    }
  }

  const handleDelete = async (definition: CredentialDefinition) => {
    setActionType(`delete-${definition.type}`)
    try {
      const next = await apiService.deleteProjectMobileCredential(projectId, definition.type)
      onCredentialsChange(next)
      clearValues(definition.type)
      if (expandedType === definition.type) {
        setExpandedType(null)
      }
      addNotification({
        type: 'success',
        title: 'Mobile credential deleted',
        message: `${definition.label} was removed from this project.`,
      })
    } catch (error) {
      addNotification({
        type: 'error',
        title: 'Credential delete failed',
        message: getApiErrorMessage(error, `Unable to delete ${definition.label}.`),
      })
    } finally {
      setActionType(null)
    }
  }

  return (
    <div className="mt-5 rounded-2xl border border-white/8 bg-black/24 p-4" data-testid="mobile-credential-panel">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.22em] text-blue-200/80">
            <Lock className="h-3.5 w-3.5" />
            Mobile Credentials
          </div>
          <p className="mt-2 max-w-2xl text-xs leading-5 text-gray-400">
            Values are encrypted server-side and never returned here. Use scoped API keys; do not paste Apple ID passwords or personal account passwords.
          </p>
        </div>
        <span className={cn(
          'rounded-full border px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.16em]',
          credentials?.complete
            ? 'border-emerald-300/20 bg-emerald-300/10 text-emerald-100'
            : credentials?.present?.length
              ? 'border-amber-300/20 bg-amber-300/10 text-amber-100'
              : 'border-white/10 bg-white/[0.03] text-gray-300',
        )}>
          {credentials?.status || 'not checked'}
        </span>
      </div>

      <div className="mt-4 grid gap-3">
        {credentialDefinitions.map((definition) => {
          const isPresent = present.has(definition.type)
          const isMissing = missing.has(definition.type)
          const isRequired = required.has(definition.type)
          const metadata = metadataByType.get(definition.type)
          const expanded = expandedType === definition.type
          const values = valuesByType[definition.type]
          const canSave = isCredentialComplete(definition, values) && !actionType
          return (
            <div
              key={definition.type}
              className="rounded-2xl border border-white/8 bg-black/22 px-4 py-4"
              data-testid={`mobile-credential-${definition.type}`}
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="text-sm font-semibold text-white">{definition.label}</div>
                    {isRequired ? (
                      <span className="rounded-full border border-blue-300/16 bg-blue-300/8 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-blue-100">
                        Required
                      </span>
                    ) : (
                      <span className="rounded-full border border-white/10 bg-white/[0.03] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-gray-400">
                        Optional
                      </span>
                    )}
                    <span className={cn(
                      'rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em]',
                      isPresent
                        ? 'border-emerald-300/18 bg-emerald-300/8 text-emerald-100'
                        : isMissing
                          ? 'border-amber-300/18 bg-amber-300/8 text-amber-100'
                          : 'border-white/10 bg-white/[0.03] text-gray-400',
                    )}>
                      {isPresent ? 'Stored' : isMissing ? 'Missing' : 'Not stored'}
                    </span>
                  </div>
                  <p className="mt-2 text-xs leading-5 text-gray-400">{definition.summary}</p>
                  {metadata ? (
                    <div className="mt-2 flex items-center gap-2 text-xs text-emerald-100/80">
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      Stored {formatRelativeTime(metadata.updated_at)}
                    </div>
                  ) : null}
                </div>
                <div className="flex shrink-0 flex-wrap gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      if (expanded) {
                        clearValues(definition.type)
                        setExpandedType(null)
                      } else {
                        setExpandedType(definition.type)
                      }
                    }}
                    disabled={Boolean(actionType)}
                    className="rounded-xl border border-blue-300/14 bg-blue-300/8 px-3 text-xs text-blue-50"
                  >
                    <KeyRound className="mr-2 h-3.5 w-3.5" />
                    {isPresent ? 'Replace' : 'Add'}
                  </Button>
                  {isPresent ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => void handleDelete(definition)}
                      disabled={Boolean(actionType)}
                      className="rounded-xl border border-red-300/14 bg-red-300/8 px-3 text-xs text-red-50"
                    >
                      <Trash2 className="mr-2 h-3.5 w-3.5" />
                      Delete
                    </Button>
                  ) : null}
                </div>
              </div>

              {expanded ? (
                <div className="mt-4 grid gap-3 border-t border-white/8 pt-4">
                  {definition.fields.map((field) => {
                    const commonClass = 'w-full rounded-xl border border-white/10 bg-black/32 px-3 py-2 text-sm text-white placeholder:text-gray-600 outline-none transition focus:border-blue-300/40 focus:ring-2 focus:ring-blue-300/10'
                    const value = fieldValue(valuesByType, definition.type, field.key)
                    return (
                      <label key={field.key} className="block">
                        <span className="text-xs font-medium text-gray-300">{field.label}</span>
                        {field.kind === 'textarea' ? (
                          <textarea
                            value={value}
                            onChange={(event) => updateValue(definition.type, field.key, event.target.value)}
                            placeholder={field.placeholder}
                            rows={field.key.includes('json') || field.key.includes('private_key') ? 5 : 3}
                            className={cn(commonClass, 'mt-1 resize-y font-mono text-xs leading-5')}
                          />
                        ) : (
                          <input
                            type={field.kind === 'password' ? 'password' : 'text'}
                            value={value}
                            onChange={(event) => updateValue(definition.type, field.key, event.target.value)}
                            placeholder={field.placeholder}
                            autoComplete="off"
                            className={cn(commonClass, 'mt-1')}
                          />
                        )}
                      </label>
                    )
                  })}
                  <div className="flex flex-wrap justify-end gap-2">
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => {
                        clearValues(definition.type)
                        setExpandedType(null)
                      }}
                      disabled={Boolean(actionType)}
                      className="rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-gray-100"
                    >
                      Cancel
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => void handleSave(definition)}
                      disabled={!canSave}
                      className="rounded-xl border border-emerald-300/18 bg-emerald-300/14 px-3 text-xs text-emerald-50 hover:bg-emerald-300/20 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {actionType === `save-${definition.type}` ? 'Saving...' : 'Save encrypted credential'}
                    </Button>
                  </div>
                </div>
              ) : null}
            </div>
          )
        })}
      </div>
    </div>
  )
}

export default MobileCredentialPanel
