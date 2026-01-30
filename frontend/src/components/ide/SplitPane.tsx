// APEX.BUILD Split Pane Component
// Recursive split pane with drag-to-resize dividers

import React, { useCallback, lazy, Suspense, forwardRef } from 'react'
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels'
import { cn } from '@/lib/utils'
import { Pane, OpenFile, PaneManager } from '@/hooks/usePaneManager'
import { File, AICapability } from '@/types'
import { Button, Loading } from '@/components/ui'
import {
  X,
  SplitSquareHorizontal,
  SplitSquareVertical,
  MoreVertical,
  Code,
  GripVertical,
  GripHorizontal,
} from 'lucide-react'

// Lazy load Monaco Editor for performance
const MonacoEditor = lazy(() => import('@/components/editor/MonacoEditor').then(m => ({ default: m.MonacoEditor })))

// Loading fallback for lazy-loaded editor
const EditorLoadingFallback = () => (
  <div className="flex items-center justify-center h-full bg-gray-900/50">
    <div className="text-center">
      <Loading size="lg" variant="primary" />
      <p className="mt-3 text-sm text-gray-400">Loading editor...</p>
    </div>
  </div>
)

interface SplitPaneProps {
  pane: Pane
  paneManager: PaneManager
  isActive: boolean
  onFileSelect?: (file: File) => void
  onFileSave?: (fileId: number, content: string) => Promise<void>
  onAIRequest?: (capability: AICapability, prompt: string, code: string) => void
  editorRefs?: React.MutableRefObject<Map<string, any>>
}

// Resize handle component
const ResizeHandle: React.FC<{
  direction: 'horizontal' | 'vertical'
  className?: string
}> = ({ direction, className }) => {
  return (
    <PanelResizeHandle
      className={cn(
        'group relative flex items-center justify-center transition-all',
        direction === 'horizontal'
          ? 'w-1 hover:w-2 hover:bg-red-500/30 active:bg-red-500/50 cursor-col-resize'
          : 'h-1 hover:h-2 hover:bg-red-500/30 active:bg-red-500/50 cursor-row-resize',
        'bg-gray-700/50',
        className
      )}
    >
      <div
        className={cn(
          'absolute opacity-0 group-hover:opacity-100 transition-opacity',
          direction === 'horizontal' ? 'py-2' : 'px-2'
        )}
      >
        {direction === 'horizontal' ? (
          <GripVertical className="w-3 h-6 text-gray-400" />
        ) : (
          <GripHorizontal className="w-6 h-3 text-gray-400" />
        )}
      </div>
    </PanelResizeHandle>
  )
}

// File tab component
interface FileTabProps {
  openFile: OpenFile
  isActive: boolean
  paneId: string
  paneManager: PaneManager
}

const FileTab: React.FC<FileTabProps> = ({
  openFile,
  isActive,
  paneId,
  paneManager,
}) => {
  const handleClick = () => {
    paneManager.setActiveFileInPane(paneId, openFile.id)
  }

  const handleClose = (e: React.MouseEvent) => {
    e.stopPropagation()
    paneManager.closeFileInPane(paneId, openFile.id)
  }

  const getFileExtension = (name: string) => {
    return name.split('.').pop()?.toUpperCase() || 'TXT'
  }

  return (
    <div
      className={cn(
        'flex items-center gap-2 px-3 py-2 cursor-pointer transition-colors group shrink-0',
        isActive
          ? 'bg-gray-800 border-b-2 border-red-500'
          : 'hover:bg-gray-800/50 border-b-2 border-transparent'
      )}
      onClick={handleClick}
    >
      <span className="text-xs text-gray-500">{getFileExtension(openFile.file.name)}</span>
      <span
        className={cn(
          'text-sm truncate max-w-[120px]',
          isActive ? 'text-white' : 'text-gray-400'
        )}
      >
        {openFile.file.name}
      </span>
      {openFile.isDirty && (
        <div className="w-1.5 h-1.5 bg-red-500 rounded-full shrink-0" />
      )}
      <button
        onClick={handleClose}
        className="ml-1 opacity-0 group-hover:opacity-100 hover:bg-gray-700 rounded transition-all p-1"
      >
        <X size={12} />
      </button>
    </div>
  )
}

// Pane header with tabs and actions
interface PaneHeaderProps {
  pane: Pane
  paneManager: PaneManager
  isActive: boolean
}

const PaneHeader: React.FC<PaneHeaderProps> = ({ pane, paneManager, isActive }) => {
  const handleSplitHorizontal = () => {
    paneManager.focusPane(pane.id)
    paneManager.splitHorizontal()
  }

  const handleSplitVertical = () => {
    paneManager.focusPane(pane.id)
    paneManager.splitVertical()
  }

  const handleClose = () => {
    paneManager.closePane(pane.id)
  }

  return (
    <div
      className={cn(
        'flex items-center justify-between bg-gray-900/80 border-b transition-colors',
        isActive ? 'border-red-500/50' : 'border-gray-700/50'
      )}
    >
      {/* File tabs */}
      <div className="flex items-center overflow-x-auto scrollbar-hide flex-1">
        {(pane.openFiles || []).map(openFile => (
          <FileTab
            key={openFile.id}
            openFile={openFile}
            isActive={pane.activeFileId === openFile.id}
            paneId={pane.id}
            paneManager={paneManager}
          />
        ))}
      </div>

      {/* Pane actions */}
      <div className="flex items-center gap-1 px-2 shrink-0">
        {paneManager.canSplit && (
          <>
            <Button
              size="xs"
              variant="ghost"
              onClick={handleSplitHorizontal}
              icon={<SplitSquareHorizontal size={14} />}
              title="Split Horizontal (Cmd+\\)"
              className="opacity-60 hover:opacity-100"
            />
            <Button
              size="xs"
              variant="ghost"
              onClick={handleSplitVertical}
              icon={<SplitSquareVertical size={14} />}
              title="Split Vertical (Cmd+Shift+\\)"
              className="opacity-60 hover:opacity-100"
            />
          </>
        )}
        {paneManager.canClosePane && (
          <Button
            size="xs"
            variant="ghost"
            onClick={handleClose}
            icon={<X size={14} />}
            title="Close Pane"
            className="opacity-60 hover:opacity-100"
          />
        )}
      </div>
    </div>
  )
}

// Empty pane placeholder
const EmptyPane: React.FC<{ paneId: string; paneManager: PaneManager }> = ({
  paneId,
  paneManager,
}) => {
  return (
    <div className="h-full flex items-center justify-center text-center p-4 bg-gray-900/30">
      <div>
        <Code className="w-12 h-12 text-gray-600 mx-auto mb-4" />
        <h3 className="text-base font-semibold text-gray-400 mb-2">No file open</h3>
        <p className="text-sm text-gray-500">Select a file from the explorer to start editing</p>
        {paneManager.canSplit && (
          <div className="mt-4 flex items-center justify-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                paneManager.focusPane(paneId)
                paneManager.splitHorizontal()
              }}
              icon={<SplitSquareHorizontal size={14} />}
            >
              Split
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

// Leaf pane with editor
interface LeafPaneProps {
  pane: Pane
  paneManager: PaneManager
  isActive: boolean
  onFileSave?: (fileId: number, content: string) => Promise<void>
  onAIRequest?: (capability: AICapability, prompt: string, code: string) => void
  editorRefs?: React.MutableRefObject<Map<string, any>>
}

const LeafPane: React.FC<LeafPaneProps> = ({
  pane,
  paneManager,
  isActive,
  onFileSave,
  onAIRequest,
  editorRefs,
}) => {
  const activeOpenFile = (pane.openFiles || []).find(f => f.id === pane.activeFileId)

  const handleChange = useCallback((content: string) => {
    if (pane.activeFileId) {
      paneManager.updateFileContent(pane.id, pane.activeFileId, content)
    }
  }, [pane.id, pane.activeFileId, paneManager])

  const handleSave = useCallback(async (content: string) => {
    if (pane.activeFileId && onFileSave) {
      await onFileSave(pane.activeFileId, content)
      paneManager.markFileSaved(pane.id, pane.activeFileId)
    }
  }, [pane.id, pane.activeFileId, paneManager, onFileSave])

  const handleFocus = useCallback(() => {
    paneManager.focusPane(pane.id)
  }, [pane.id, paneManager])

  const setEditorRef = useCallback((editor: any) => {
    if (editorRefs && editor) {
      editorRefs.current.set(pane.id, editor)
    }
  }, [pane.id, editorRefs])

  return (
    <div
      className={cn(
        'h-full flex flex-col overflow-hidden',
        isActive && 'ring-1 ring-red-500/30'
      )}
      onClick={handleFocus}
    >
      <PaneHeader pane={pane} paneManager={paneManager} isActive={isActive} />

      <div className="flex-1 min-h-0 overflow-hidden">
        {activeOpenFile ? (
          <Suspense fallback={<EditorLoadingFallback />}>
            <MonacoEditor
              ref={setEditorRef}
              file={activeOpenFile.file}
              value={activeOpenFile.content}
              onChange={handleChange}
              onSave={handleSave}
              onAIRequest={onAIRequest}
              height="100%"
              showAIPanel={isActive}
            />
          </Suspense>
        ) : (
          <EmptyPane paneId={pane.id} paneManager={paneManager} />
        )}
      </div>
    </div>
  )
}

// Main split pane component - renders recursively
export const SplitPane: React.FC<SplitPaneProps> = ({
  pane,
  paneManager,
  isActive,
  onFileSelect,
  onFileSave,
  onAIRequest,
  editorRefs,
}) => {
  // If this is a split pane, render children
  if (pane.children) {
    const [first, second] = pane.children
    const direction = pane.direction || 'horizontal'

    return (
      <PanelGroup
        direction={direction}
        className="h-full"
      >
        <Panel
          defaultSize={pane.splitRatio || 50}
          minSize={15}
          className="overflow-hidden"
        >
          <SplitPane
            pane={first}
            paneManager={paneManager}
            isActive={paneManager.activePaneId === first.id || (first.children && isActive)}
            onFileSelect={onFileSelect}
            onFileSave={onFileSave}
            onAIRequest={onAIRequest}
            editorRefs={editorRefs}
          />
        </Panel>

        <ResizeHandle direction={direction} />

        <Panel
          minSize={15}
          className="overflow-hidden"
        >
          <SplitPane
            pane={second}
            paneManager={paneManager}
            isActive={paneManager.activePaneId === second.id || (second.children && isActive)}
            onFileSelect={onFileSelect}
            onFileSave={onFileSave}
            onAIRequest={onAIRequest}
            editorRefs={editorRefs}
          />
        </Panel>
      </PanelGroup>
    )
  }

  // This is a leaf pane - render the editor
  return (
    <LeafPane
      pane={pane}
      paneManager={paneManager}
      isActive={isActive}
      onFileSave={onFileSave}
      onAIRequest={onAIRequest}
      editorRefs={editorRefs}
    />
  )
}

export default SplitPane
