// APEX.BUILD Diff Viewer
// Side-by-side code comparison using Monaco Diff Editor

import React, { useEffect, useRef, useState } from 'react'
import * as monaco from 'monaco-editor'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { Button, Badge } from '@/components/ui'
import { X, ArrowRight } from 'lucide-react'

interface DiffViewerProps {
  originalContent: string
  modifiedContent: string
  originalLabel?: string
  modifiedLabel?: string
  language: string
  onClose: () => void
  className?: string
}

export const DiffViewer: React.FC<DiffViewerProps> = ({
  originalContent,
  modifiedContent,
  originalLabel = 'Original',
  modifiedLabel = 'Modified',
  language,
  onClose,
  className
}) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const diffEditorRef = useRef<monaco.editor.IStandaloneDiffEditor | null>(null)
  const { currentTheme } = useStore()

  // Initialize Diff Editor
  useEffect(() => {
    if (!containerRef.current) return

    // Create models
    const originalModel = monaco.editor.createModel(originalContent, language)
    const modifiedModel = monaco.editor.createModel(modifiedContent, language)

    // Create editor
    const diffEditor = monaco.editor.createDiffEditor(containerRef.current, {
      originalEditable: false,
      readOnly: true,
      theme: `apex-${currentTheme.id}`, // Reuse theme from MonacoEditor setup
      automaticLayout: true,
      renderSideBySide: true,
      fontSize: 14,
      fontFamily: '"Fira Code", "SF Mono", Monaco, Menlo, Consolas, monospace',
      minimap: { enabled: false },
    })

    diffEditor.setModel({
      original: originalModel,
      modified: modifiedModel
    })

    diffEditorRef.current = diffEditor

    return () => {
      diffEditor.dispose()
      originalModel.dispose()
      modifiedModel.dispose()
    }
  }, [originalContent, modifiedContent, language, currentTheme.id])

  return (
    <div className={cn("flex flex-col h-full bg-gray-950", className)}>
      {/* Header */}
      <div className="h-10 bg-gray-900 border-b border-gray-800 flex items-center justify-between px-4">
        <div className="flex items-center gap-4 text-sm">
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="bg-red-900/20 text-red-400 border-red-900/50">
              {originalLabel}
            </Badge>
          </div>
          <ArrowRight className="w-4 h-4 text-gray-600" />
          <div className="flex items-center gap-2">
            <Badge variant="outline" className="bg-green-900/20 text-green-400 border-green-900/50">
              {modifiedLabel}
            </Badge>
          </div>
        </div>
        <Button
          size="sm"
          variant="ghost"
          onClick={onClose}
          icon={<X size={14} />}
        >
          Close Diff
        </Button>
      </div>

      {/* Editor Container */}
      <div ref={containerRef} className="flex-1" />
    </div>
  )
}
