import React, { useMemo, useState } from 'react'
import { FileText } from 'lucide-react'

import {
  LEGAL_DOCUMENTS,
  LEGAL_POLICY_VERSION,
  type LegalDocumentId,
} from './legalDocumentsData'

type LegalDocumentsProps = {
  initialDocumentId?: LegalDocumentId
  compact?: boolean
}

export const LegalDocumentLinks: React.FC<{
  onSelect: (documentId: LegalDocumentId) => void
  className?: string
}> = ({ onSelect, className = '' }) => {
  return (
    <div className={`flex flex-wrap gap-2 ${className}`.trim()}>
      {LEGAL_DOCUMENTS.map((document) => (
        <button
          key={document.id}
          type="button"
          onClick={() => onSelect(document.id)}
          className="rounded-full border border-red-500/30 bg-red-500/10 px-3 py-1 text-xs font-semibold text-red-200 transition hover:border-red-400 hover:bg-red-500/20"
        >
          {document.title}
        </button>
      ))}
    </div>
  )
}

export const LegalDocuments: React.FC<LegalDocumentsProps> = ({
  initialDocumentId = 'terms',
  compact = false,
}) => {
  const initialDocument = useMemo(
    () => LEGAL_DOCUMENTS.find((document) => document.id === initialDocumentId) || LEGAL_DOCUMENTS[0],
    [initialDocumentId]
  )
  const [selectedDocumentId, setSelectedDocumentId] = useState<LegalDocumentId>(initialDocument.id)

  const selectedDocument = LEGAL_DOCUMENTS.find((document) => document.id === selectedDocumentId) || LEGAL_DOCUMENTS[0]
  const SelectedIcon = selectedDocument.icon

  return (
    <div className={`rounded-2xl border border-gray-800 bg-gray-950/80 ${compact ? 'p-4' : 'p-6'}`}>
      <div className={`flex ${compact ? 'flex-col gap-4' : 'flex-col gap-6'}`}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-red-300/80">Legal</p>
            <h3 className={`${compact ? 'mt-1 text-xl' : 'mt-2 text-2xl'} font-black text-white`}>
              Terms, privacy, and platform policies
            </h3>
            <p className="mt-2 max-w-2xl text-sm text-gray-400">
              Effective {LEGAL_POLICY_VERSION}. These documents govern access to APEX.BUILD and are incorporated into account registration and continued platform use.
            </p>
          </div>
          <div className="rounded-xl border border-red-500/20 bg-red-500/10 px-3 py-2 text-xs text-red-100">
            Version {LEGAL_POLICY_VERSION}
          </div>
        </div>

        <div className={`${compact ? 'space-y-3' : 'grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]'}`}>
          <div className="space-y-2">
            {LEGAL_DOCUMENTS.map((document) => {
              const Icon = document.icon
              const isSelected = document.id === selectedDocument.id
              return (
                <button
                  key={document.id}
                  type="button"
                  onClick={() => setSelectedDocumentId(document.id)}
                  className={`w-full rounded-xl border p-4 text-left transition ${
                    isSelected
                      ? 'border-red-500/50 bg-red-500/10 shadow-[0_0_0_1px_rgba(239,68,68,0.2)]'
                      : 'border-gray-800 bg-black/40 hover:border-red-500/30 hover:bg-red-500/5'
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <Icon className={`mt-0.5 h-5 w-5 ${isSelected ? 'text-red-300' : 'text-gray-500'}`} />
                    <div>
                      <div className={`font-semibold ${isSelected ? 'text-white' : 'text-gray-200'}`}>{document.title}</div>
                      <p className="mt-1 text-xs leading-5 text-gray-400">{document.summary}</p>
                    </div>
                  </div>
                </button>
              )
            })}
          </div>

          <div className="rounded-2xl border border-gray-800 bg-black/50 p-5">
            <div className="flex items-start gap-3">
              <div className="rounded-xl border border-red-500/20 bg-red-500/10 p-2">
                <SelectedIcon className="h-5 w-5 text-red-300" />
              </div>
              <div>
                <h4 className="text-xl font-bold text-white">{selectedDocument.title}</h4>
                <p className="mt-1 text-sm text-gray-400">{selectedDocument.summary}</p>
              </div>
            </div>

            <div className="mt-6 space-y-6">
              {selectedDocument.sections.map((section) => (
                <section key={section.heading} className="space-y-3">
                  <div className="flex items-center gap-2">
                    <FileText className="h-4 w-4 text-red-300" />
                    <h5 className="text-sm font-semibold uppercase tracking-[0.18em] text-red-200">{section.heading}</h5>
                  </div>
                  <div className="space-y-3">
                    {section.paragraphs.map((paragraph) => (
                      <p key={paragraph} className="text-sm leading-7 text-gray-300">
                        {paragraph}
                      </p>
                    ))}
                  </div>
                </section>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default LegalDocuments
