import React, { useState } from 'react'
import { Check, X, CheckCheck, XCircle, RefreshCw } from 'lucide-react'
import { DiffViewer } from '@/components/ide/DiffViewer'
import apiService from '@/services/api'

interface ProposedEdit {
  id: string
  build_id: string
  agent_id: string
  agent_role: string
  file_path: string
  original_content: string
  proposed_content: string
  language: string
  status: 'pending' | 'approved' | 'rejected'
}

interface DiffReviewPanelProps {
  buildId: string
  edits: ProposedEdit[]
  onEditsUpdated: () => void
  onClose: () => void
}

export const DiffReviewPanel: React.FC<DiffReviewPanelProps> = ({
  buildId,
  edits,
  onEditsUpdated,
  onClose,
}) => {
  const [selectedEdit, setSelectedEdit] = useState<ProposedEdit | null>(edits[0] || null)
  const [processing, setProcessing] = useState(false)

  const pendingEdits = edits.filter(e => e.status === 'pending')

  const handleApprove = async (editIds: string[]) => {
    setProcessing(true)
    try {
      await apiService.post(`/build/${buildId}/approve-edits`, { edit_ids: editIds })
      onEditsUpdated()
    } catch (err) {
      console.error('Failed to approve edits:', err)
    } finally {
      setProcessing(false)
    }
  }

  const handleReject = async (editIds: string[]) => {
    setProcessing(true)
    try {
      await apiService.post(`/build/${buildId}/reject-edits`, { edit_ids: editIds })
      onEditsUpdated()
    } catch (err) {
      console.error('Failed to reject edits:', err)
    } finally {
      setProcessing(false)
    }
  }

  const handleApproveAll = async () => {
    setProcessing(true)
    try {
      await apiService.post(`/build/${buildId}/approve-all`)
      onEditsUpdated()
    } catch (err) {
      console.error('Failed to approve all:', err)
    } finally {
      setProcessing(false)
    }
  }

  const handleRejectAll = async () => {
    setProcessing(true)
    try {
      await apiService.post(`/build/${buildId}/reject-all`)
      onEditsUpdated()
    } catch (err) {
      console.error('Failed to reject all:', err)
    } finally {
      setProcessing(false)
    }
  }

  return (
    <div className="flex flex-col h-full bg-gray-950">
      {/* Header */}
      <div className="h-12 bg-gray-900 border-b border-gray-800 flex items-center justify-between px-4">
        <div className="flex items-center gap-3">
          <h3 className="text-sm font-bold text-white">Review Proposed Changes</h3>
          <span className="text-xs px-2 py-0.5 bg-yellow-900/30 text-yellow-400 rounded-full">
            {pendingEdits.length} pending
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleApproveAll}
            disabled={processing || pendingEdits.length === 0}
            className="flex items-center gap-1 px-3 py-1 bg-green-800/30 hover:bg-green-700/40 text-green-400 text-xs font-medium rounded-lg transition-colors disabled:opacity-50"
          >
            <CheckCheck size={14} />
            Approve All
          </button>
          <button
            onClick={handleRejectAll}
            disabled={processing || pendingEdits.length === 0}
            className="flex items-center gap-1 px-3 py-1 bg-red-800/30 hover:bg-red-700/40 text-red-400 text-xs font-medium rounded-lg transition-colors disabled:opacity-50"
          >
            <XCircle size={14} />
            Reject All
          </button>
          <button onClick={onClose} className="text-gray-500 hover:text-white transition-colors">
            <X size={18} />
          </button>
        </div>
      </div>

      <div className="flex flex-1 min-h-0">
        {/* File List */}
        <div className="w-64 border-r border-gray-800 overflow-y-auto">
          {edits.map(edit => (
            <button
              key={edit.id}
              onClick={() => setSelectedEdit(edit)}
              className={`w-full text-left px-3 py-2 border-b border-gray-800/50 transition-colors ${
                selectedEdit?.id === edit.id ? 'bg-gray-800' : 'hover:bg-gray-800/50'
              }`}
            >
              <div className="text-xs text-gray-300 truncate">{edit.file_path}</div>
              <div className="flex items-center gap-2 mt-1">
                <span className="text-[10px] text-gray-500">{edit.agent_role}</span>
                <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                  edit.status === 'approved' ? 'bg-green-900/30 text-green-400' :
                  edit.status === 'rejected' ? 'bg-red-900/30 text-red-400' :
                  'bg-yellow-900/30 text-yellow-400'
                }`}>
                  {edit.status}
                </span>
              </div>
            </button>
          ))}
        </div>

        {/* Diff View */}
        <div className="flex-1 flex flex-col">
          {selectedEdit ? (
            <>
              <DiffViewer
                originalContent={selectedEdit.original_content}
                modifiedContent={selectedEdit.proposed_content}
                originalLabel="Original"
                modifiedLabel={`Proposed (${selectedEdit.agent_role})`}
                language={selectedEdit.language}
                onClose={() => setSelectedEdit(null)}
                className="flex-1"
              />
              {selectedEdit.status === 'pending' && (
                <div className="h-12 bg-gray-900 border-t border-gray-800 flex items-center justify-end gap-2 px-4">
                  <button
                    onClick={() => handleReject([selectedEdit.id])}
                    disabled={processing}
                    className="flex items-center gap-1 px-4 py-1.5 bg-red-900/30 hover:bg-red-800/50 text-red-400 text-sm font-medium rounded-lg transition-colors"
                  >
                    <X size={14} />
                    Reject
                  </button>
                  <button
                    onClick={() => handleApprove([selectedEdit.id])}
                    disabled={processing}
                    className="flex items-center gap-1 px-4 py-1.5 bg-green-700 hover:bg-green-600 text-white text-sm font-medium rounded-lg transition-colors"
                  >
                    <Check size={14} />
                    Approve
                  </button>
                </div>
              )}
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center text-gray-500">
              Select a file to review changes
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default DiffReviewPanel
