// Split Pane Editor - APEX.BUILD
// Multi-pane code editor with resizable splits

import React, { lazy, Suspense, useCallback, useRef, useImperativeHandle, forwardRef } from 'react'
import { Panel, Group as PanelGroup, Separator as PanelResizeHandle } from 'react-resizable-panels'
import { cn } from '@/lib/utils'
import { EditorPane, PaneFile, PaneLayout } from '@/hooks/usePaneManager'
import { AICapability, File } from '@/types'
import { Button, Loading } from '@/components/ui'
import {
  X,
  Code,
  SplitSquareHorizontal,
  SplitSquareVertical,
  Columns,
  XCircle
} from 'lucide-react'

// Lazy load Monaco Editor
const MonacoEditor = lazy(() =>
  import('@/components/editor/MonacoEditor').then(m => ({ default: m.MonacoEditor }))
)

const EditorLoadingFallback = () => (
  <div className="flex items-center justify-center h-full bg-gray-900/50">
    <div className="text-center">
      <Loading size="lg" variant="primary" />
      <p className="mt-3 text-sm text-gray-400">Loading editor...</p>
    </div>
  </div>
)

interface EditorPaneContentProps {
  pane: EditorPane
  isActive: boolean
  onFocus: () => void
  onFileSelect: (fileId: number) => void
  onFileClose: (fileId: number) => void
  onFileChange: (content: string) => void
  onFileSave: (content: string) => void
  onAIRequest: (capability: AICapability, prompt: string, code: string) => Promise<any>
  onClosePane?: () => void
  canClosePane: boolean
  editorRef?: (el: any) => void
}

const EditorPaneContent: React.FC<EditorPaneContentProps> = ({
  pane,
  isActive,
  onFocus,
  onFileSelect,
  onFileClose,
  onFileChange,
  onFileSave,
  onAIRequest,
  onClosePane,
  canClosePane,
  editorRef
}) => {
  const activeFile = pane.files.find(f => f.file.id === pane.activeFileId)

  return (
    <div
      className={cn(
        'h-full flex flex-col bg-gray-950 border border-gray-800 rounded-sm overflow-hidden',
        isActive && 'ring-1 ring-cyan-500/50'
      )}
      onClick={onFocus}
    >
      {/* File tabs */}
      {pane.files.length > 0 && (
        <div className="flex items-center bg-gray-900/80 border-b border-gray-700/50 min-h-[36px]">
          <div className="flex-1 flex items-center overflow-x-auto scrollbar-hide">
            {pane.files.map(paneFile => {
              const isFileActive = pane.activeFileId === paneFile.file.id

              return (
                <div
                  key={paneFile.file.id}
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-1.5 cursor-pointer transition-colors group shrink-0',
                    isFileActive
                      ? 'bg-gray-800 border-b-2 border-cyan-500'
                      : 'hover:bg-gray-800/50'
                  )}
                  onClick={(e) => {
                    e.stopPropagation()
                    onFileSelect(paneFile.file.id)
                  }}
                >
                  <span className="text-[10px] text-gray-500 uppercase">
                    {paneFile.file.name.split('.').pop()}
                  </span>
                  <span className={cn(
                    'text-xs truncate max-w-[120px]',
                    isFileActive ? 'text-white' : 'text-gray-400'
                  )}>
                    {paneFile.file.name}
                  </span>
                  {paneFile.hasUnsavedChanges && (
                    <div className="w-1.5 h-1.5 bg-orange-500 rounded-full" />
                  )}
                  <button
                    onClick={(e) => {
                      e.stopPropagation()
                      onFileClose(paneFile.file.id)
                    }}
                    className="ml-1 opacity-0 group-hover:opacity-100 hover:bg-gray-700 rounded p-0.5 transition-all"
                  >
                    <X size={10} />
                  </button>
                </div>
              )
            })}
          </div>

          {/* Pane close button */}
          {canClosePane && onClosePane && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                onClosePane()
              }}
              className="p-1.5 mx-1 text-gray-500 hover:text-red-400 hover:bg-gray-800 rounded transition-colors"
              title="Close pane"
            >
              <XCircle size={14} />
            </button>
          )}
        </div>
      )}

      {/* Editor */}
      <div className="flex-1 min-h-0">
        {activeFile ? (
          <Suspense fallback={<EditorLoadingFallback />}>
            <MonacoEditor
              ref={editorRef}
              file={activeFile.file}
              value={activeFile.content}
              onChange={onFileChange}
              onSave={onFileSave}
              onAIRequest={onAIRequest}
              height="100%"
            />
          </Suspense>
        ) : (
          <div className="h-full flex items-center justify-center text-center p-4">
            <div>
              <Code className="w-10 h-10 text-gray-600 mx-auto mb-3" />
              <p className="text-sm text-gray-500">No file open</p>
              <p className="text-xs text-gray-600 mt-1">Select a file from the explorer</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// Resize handle component
const ResizeHandle = ({ direction }: { direction: 'horizontal' | 'vertical' }) => (
  <PanelResizeHandle
    className={cn(
      'group relative flex items-center justify-center',
      direction === 'horizontal' ? 'w-1 cursor-col-resize' : 'h-1 cursor-row-resize',
      'hover:bg-cyan-500/30 active:bg-cyan-500/50 transition-colors'
    )}
  >
    <div
      className={cn(
        'absolute bg-gray-600 group-hover:bg-cyan-500 group-active:bg-cyan-400 transition-colors rounded-full',
        direction === 'horizontal' ? 'w-0.5 h-8' : 'w-8 h-0.5'
      )}
    />
  </PanelResizeHandle>
)

export interface SplitPaneEditorProps {
  layout: PaneLayout
  activePaneId: string | null
  canSplit: boolean
  onFocusPane: (paneId: string) => void
  onClosePane: (paneId: string) => void
  onFileSelect: (fileId: number, paneId: string) => void
  onFileClose: (fileId: number, paneId: string) => void
  onFileChange: (fileId: number, content: string, paneId: string) => void
  onFileSave: (fileId: number, content: string, paneId: string) => void
  onAIRequest: (capability: AICapability, prompt: string, code: string) => Promise<any>
  onSplitHorizontal: () => void
  onSplitVertical: () => void
}

export interface SplitPaneEditorRef {
  insertCode: (code: string) => void
  revealLine: (line: number) => void
}

export const SplitPaneEditor = forwardRef<SplitPaneEditorRef, SplitPaneEditorProps>(({
  layout,
  activePaneId,
  canSplit,
  onFocusPane,
  onClosePane,
  onFileSelect,
  onFileClose,
  onFileChange,
  onFileSave,
  onAIRequest,
  onSplitHorizontal,
  onSplitVertical
}, ref) => {
  const editorRefs = useRef<Map<string, any>>(new Map())

  useImperativeHandle(ref, () => ({
    insertCode: (code: string) => {
      if (!activePaneId) return
      const editor = editorRefs.current.get(activePaneId)
      if (editor) {
        const position = editor.getPosition()
        editor.executeEdits('ai-insert', [{
          range: {
            startLineNumber: position.lineNumber,
            startColumn: position.column,
            endLineNumber: position.lineNumber,
            endColumn: position.column
          },
          text: code
        }])
      }
    },
    revealLine: (line: number) => {
      if (!activePaneId) return
      const editor = editorRefs.current.get(activePaneId)
      if (editor) {
        editor.revealLineInCenter(line)
        editor.setPosition({ lineNumber: line, column: 1 })
        editor.focus()
      }
    }
  }))

  const renderPane = useCallback((pane: EditorPane) => (
    <EditorPaneContent
      key={pane.id}
      pane={pane}
      isActive={pane.id === activePaneId}
      onFocus={() => onFocusPane(pane.id)}
      onFileSelect={(fileId) => onFileSelect(fileId, pane.id)}
      onFileClose={(fileId) => onFileClose(fileId, pane.id)}
      onFileChange={(content) => {
        const activeFile = pane.files.find(f => f.file.id === pane.activeFileId)
        if (activeFile) {
          onFileChange(activeFile.file.id, content, pane.id)
        }
      }}
      onFileSave={(content) => {
        const activeFile = pane.files.find(f => f.file.id === pane.activeFileId)
        if (activeFile) {
          onFileSave(activeFile.file.id, content, pane.id)
        }
      }}
      onAIRequest={onAIRequest}
      onClosePane={layout.panes.length > 1 ? () => onClosePane(pane.id) : undefined}
      canClosePane={layout.panes.length > 1}
      editorRef={(el: any) => {
        if (el) editorRefs.current.set(pane.id, el)
        else editorRefs.current.delete(pane.id)
      }}
    />
  ), [activePaneId, layout.panes.length, onFocusPane, onFileSelect, onFileClose, onFileChange, onFileSave, onAIRequest, onClosePane])

  // Render layout based on type
  const renderLayout = () => {
    switch (layout.type) {
      case 'single':
        return renderPane(layout.panes[0])

      case 'horizontal':
        return (
          <PanelGroup orientation="horizontal" className="h-full">
            <Panel defaultSize={50} minSize={20}>
              {renderPane(layout.panes[0])}
            </Panel>
            <ResizeHandle direction="horizontal" />
            <Panel defaultSize={50} minSize={20}>
              {renderPane(layout.panes[1])}
            </Panel>
          </PanelGroup>
        )

      case 'vertical':
        return (
          <PanelGroup direction="vertical" className="h-full">
            <Panel defaultSize={50} minSize={15}>
              {renderPane(layout.panes[0])}
            </Panel>
            <ResizeHandle direction="vertical" />
            <Panel defaultSize={50} minSize={15}>
              {renderPane(layout.panes[1])}
            </Panel>
          </PanelGroup>
        )

      case 'grid':
        // 2x2 grid for 3-4 panes
        const [p1, p2, p3, p4] = layout.panes
        return (
          <PanelGroup direction="vertical" className="h-full">
            <Panel defaultSize={50} minSize={15}>
              <PanelGroup orientation="horizontal" className="h-full">
                <Panel defaultSize={50} minSize={20}>
                  {renderPane(p1)}
                </Panel>
                <ResizeHandle direction="horizontal" />
                <Panel defaultSize={50} minSize={20}>
                  {p2 ? renderPane(p2) : <EmptyPaneSlot />}
                </Panel>
              </PanelGroup>
            </Panel>
            <ResizeHandle direction="vertical" />
            <Panel defaultSize={50} minSize={15}>
              <PanelGroup orientation="horizontal" className="h-full">
                <Panel defaultSize={50} minSize={20}>
                  {p3 ? renderPane(p3) : <EmptyPaneSlot />}
                </Panel>
                <ResizeHandle direction="horizontal" />
                <Panel defaultSize={50} minSize={20}>
                  {p4 ? renderPane(p4) : <EmptyPaneSlot />}
                </Panel>
              </PanelGroup>
            </Panel>
          </PanelGroup>
        )

      default:
        return null
    }
  }

  return (
    <div className="h-full flex flex-col">
      {/* Split controls toolbar */}
      <div className="flex items-center justify-between bg-gray-900/60 border-b border-gray-800 px-2 py-1">
        <div className="flex items-center gap-1">
          <span className="text-xs text-gray-500 mr-2">
            {layout.panes.length} pane{layout.panes.length > 1 ? 's' : ''}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            size="xs"
            variant="ghost"
            onClick={onSplitHorizontal}
            disabled={!canSplit}
            icon={<Columns size={12} />}
            title="Split horizontal (Cmd+\)"
            className="text-gray-400 hover:text-white"
          >
            Split H
          </Button>
          <Button
            size="xs"
            variant="ghost"
            onClick={onSplitVertical}
            disabled={!canSplit}
            icon={<SplitSquareVertical size={12} />}
            title="Split vertical (Cmd+Shift+\)"
            className="text-gray-400 hover:text-white"
          >
            Split V
          </Button>
        </div>
      </div>

      {/* Editor panes */}
      <div className="flex-1 min-h-0 p-1">
        {renderLayout()}
      </div>
    </div>
  )
})

SplitPaneEditor.displayName = 'SplitPaneEditor'

// Empty slot for grid layout
const EmptyPaneSlot = () => (
  <div className="h-full flex items-center justify-center bg-gray-950/50 border border-dashed border-gray-700 rounded">
    <p className="text-xs text-gray-600">Empty pane</p>
  </div>
)

export default SplitPaneEditor