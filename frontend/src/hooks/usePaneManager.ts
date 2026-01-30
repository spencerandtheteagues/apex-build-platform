// Split Pane Manager Hook - APEX.BUILD
// Manages multiple editor panes with file tabs

import { useState, useCallback, useMemo } from 'react'
import { v4 as uuidv4 } from 'uuid'
import { File } from '@/types'

export interface PaneFile {
  file: File
  content: string
  hasUnsavedChanges: boolean
}

export interface EditorPane {
  id: string
  files: PaneFile[]
  activeFileId: number | null
}

export interface PaneLayout {
  type: 'single' | 'horizontal' | 'vertical' | 'grid'
  panes: EditorPane[]
}

export interface UsePaneManagerReturn {
  layout: PaneLayout
  activePane: EditorPane | null
  activePaneId: string | null

  // Pane operations
  splitHorizontal: () => void
  splitVertical: () => void
  closePane: (paneId: string) => void
  focusPane: (paneId: string) => void

  // File operations within panes
  openFile: (file: File, paneId?: string) => void
  closeFile: (fileId: number, paneId?: string) => void
  setActiveFile: (fileId: number, paneId?: string) => void
  updateFileContent: (fileId: number, content: string, paneId?: string) => void
  markFileSaved: (fileId: number, paneId?: string) => void

  // Get pane data
  getPane: (paneId: string) => EditorPane | undefined
  getPaneFiles: (paneId: string) => PaneFile[]
  getPaneActiveFile: (paneId: string) => PaneFile | undefined

  // Layout info
  canSplit: boolean
  paneCount: number
}

const MAX_PANES = 4

const createPane = (): EditorPane => ({
  id: uuidv4(),
  files: [],
  activeFileId: null
})

export function usePaneManager(): UsePaneManagerReturn {
  const [layout, setLayout] = useState<PaneLayout>({
    type: 'single',
    panes: [createPane()]
  })
  const [activePaneId, setActivePaneId] = useState<string | null>(layout.panes[0]?.id || null)

  const canSplit = layout.panes.length < MAX_PANES
  const paneCount = layout.panes.length

  const activePane = useMemo(() => {
    return layout.panes.find(p => p.id === activePaneId) || null
  }, [layout.panes, activePaneId])

  // Split operations
  const splitHorizontal = useCallback(() => {
    if (!canSplit) return

    setLayout(prev => {
      const newPane = createPane()
      const newPanes = [...prev.panes, newPane]

      let newType: PaneLayout['type']
      if (newPanes.length === 2) {
        newType = 'horizontal'
      } else if (newPanes.length <= 4) {
        newType = 'grid'
      } else {
        return prev
      }

      return { type: newType, panes: newPanes }
    })
  }, [canSplit])

  const splitVertical = useCallback(() => {
    if (!canSplit) return

    setLayout(prev => {
      const newPane = createPane()
      const newPanes = [...prev.panes, newPane]

      let newType: PaneLayout['type']
      if (newPanes.length === 2) {
        newType = 'vertical'
      } else if (newPanes.length <= 4) {
        newType = 'grid'
      } else {
        return prev
      }

      return { type: newType, panes: newPanes }
    })
  }, [canSplit])

  const closePane = useCallback((paneId: string) => {
    setLayout(prev => {
      if (prev.panes.length <= 1) return prev

      const newPanes = prev.panes.filter(p => p.id !== paneId)
      let newType: PaneLayout['type'] = 'single'

      if (newPanes.length === 2) {
        newType = prev.type === 'grid' ? 'horizontal' : prev.type
      } else if (newPanes.length > 2) {
        newType = 'grid'
      }

      return { type: newType, panes: newPanes }
    })

    // Update active pane if needed
    setActivePaneId(prev => {
      if (prev === paneId) {
        return layout.panes.find(p => p.id !== paneId)?.id || null
      }
      return prev
    })
  }, [layout.panes])

  const focusPane = useCallback((paneId: string) => {
    setActivePaneId(paneId)
  }, [])

  // File operations
  const openFile = useCallback((file: File, paneId?: string) => {
    const targetPaneId = paneId || activePaneId
    if (!targetPaneId) return

    setLayout(prev => ({
      ...prev,
      panes: prev.panes.map(pane => {
        if (pane.id !== targetPaneId) return pane

        // Check if file is already open
        const existingFile = pane.files.find(f => f.file.id === file.id)
        if (existingFile) {
          return { ...pane, activeFileId: file.id }
        }

        // Add new file
        const newFile: PaneFile = {
          file,
          content: file.content || '',
          hasUnsavedChanges: false
        }

        return {
          ...pane,
          files: [...pane.files, newFile],
          activeFileId: file.id
        }
      })
    }))
  }, [activePaneId])

  const closeFile = useCallback((fileId: number, paneId?: string) => {
    const targetPaneId = paneId || activePaneId
    if (!targetPaneId) return

    setLayout(prev => ({
      ...prev,
      panes: prev.panes.map(pane => {
        if (pane.id !== targetPaneId) return pane

        const newFiles = pane.files.filter(f => f.file.id !== fileId)
        let newActiveFileId = pane.activeFileId

        // Update active file if we closed it
        if (pane.activeFileId === fileId) {
          const closedIndex = pane.files.findIndex(f => f.file.id === fileId)
          const nextFile = newFiles[Math.min(closedIndex, newFiles.length - 1)]
          newActiveFileId = nextFile?.file.id || null
        }

        return {
          ...pane,
          files: newFiles,
          activeFileId: newActiveFileId
        }
      })
    }))
  }, [activePaneId])

  const setActiveFile = useCallback((fileId: number, paneId?: string) => {
    const targetPaneId = paneId || activePaneId
    if (!targetPaneId) return

    setLayout(prev => ({
      ...prev,
      panes: prev.panes.map(pane => {
        if (pane.id !== targetPaneId) return pane
        return { ...pane, activeFileId: fileId }
      })
    }))
  }, [activePaneId])

  const updateFileContent = useCallback((fileId: number, content: string, paneId?: string) => {
    const targetPaneId = paneId || activePaneId
    if (!targetPaneId) return

    setLayout(prev => ({
      ...prev,
      panes: prev.panes.map(pane => {
        if (pane.id !== targetPaneId) return pane

        return {
          ...pane,
          files: pane.files.map(f => {
            if (f.file.id !== fileId) return f
            return { ...f, content, hasUnsavedChanges: true }
          })
        }
      })
    }))
  }, [activePaneId])

  const markFileSaved = useCallback((fileId: number, paneId?: string) => {
    const targetPaneId = paneId || activePaneId
    if (!targetPaneId) return

    setLayout(prev => ({
      ...prev,
      panes: prev.panes.map(pane => {
        if (pane.id !== targetPaneId) return pane

        return {
          ...pane,
          files: pane.files.map(f => {
            if (f.file.id !== fileId) return f
            return { ...f, hasUnsavedChanges: false }
          })
        }
      })
    }))
  }, [activePaneId])

  // Getters
  const getPane = useCallback((paneId: string) => {
    return layout.panes.find(p => p.id === paneId)
  }, [layout.panes])

  const getPaneFiles = useCallback((paneId: string) => {
    return layout.panes.find(p => p.id === paneId)?.files || []
  }, [layout.panes])

  const getPaneActiveFile = useCallback((paneId: string) => {
    const pane = layout.panes.find(p => p.id === paneId)
    if (!pane || !pane.activeFileId) return undefined
    return pane.files.find(f => f.file.id === pane.activeFileId)
  }, [layout.panes])

  return {
    layout,
    activePane,
    activePaneId,

    splitHorizontal,
    splitVertical,
    closePane,
    focusPane,

    openFile,
    closeFile,
    setActiveFile,
    updateFileContent,
    markFileSaved,

    getPane,
    getPaneFiles,
    getPaneActiveFile,

    canSplit,
    paneCount
  }
}

export default usePaneManager
