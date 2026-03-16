// APEX-BUILD File Tree Explorer
// Cyberpunk file system navigation with drag-drop support

import React, { useState, useCallback, useMemo } from 'react'
import { cn, getFileIcon, formatFileSize, formatRelativeTime } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { File, FileTreeNode } from '@/types'
import {
  Button,
  Badge,
  Loading,
  Card,
  Avatar,
  Input
} from '@/components/ui'
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  File as FileIcon,
  Plus,
  Search,
  X,
  MoreVertical,
  Edit3,
  Trash2,
  Copy,
  Download,
  Lock,
  Unlock,
  Eye,
  EyeOff,
  RefreshCw
} from 'lucide-react'

export interface FileTreeProps {
  className?: string
  projectId?: number
  onFileSelect?: (file: File) => void
  onFileCreate?: (parentPath: string, name: string, type: 'file' | 'directory') => void
  onFileDelete?: (file: File) => void
  onFileRename?: (file: File, newName: string) => void
  showSearch?: boolean
  showActions?: boolean
}

export const FileTree: React.FC<FileTreeProps> = ({
  className,
  projectId,
  onFileSelect,
  onFileCreate,
  onFileDelete,
  onFileRename,
  showSearch = true,
  showActions = true,
}) => {
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set(['/']))
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [contextMenu, setContextMenu] = useState<{
    file: File
    x: number
    y: number
  } | null>(null)
  const [editingFile, setEditingFile] = useState<string | null>(null)
  const [newFileName, setNewFileName] = useState('')
  const [isCreating, setIsCreating] = useState<{
    parentPath: string
    type: 'file' | 'directory'
  } | null>(null)

  const { files, isLoading, fetchFiles: refreshFiles, currentProject } = useStore()

  // Convert flat files array to tree structure
  const fileTree = useMemo(() => {
    if (!files || files.length === 0) return []

    const tree: FileTreeNode[] = []
    const pathMap: Map<string, FileTreeNode> = new Map()

    // Sort files by path to ensure parents come before children
    const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path))

    sortedFiles.forEach(file => {
      const pathParts = file.path.split('/').filter(Boolean)
      let currentPath = ''
      let currentLevel = tree

      pathParts.forEach((part, index) => {
        currentPath += `/${part}`
        const isLastPart = index === pathParts.length - 1

        let node = pathMap.get(currentPath)

        if (!node) {
          node = {
            id: currentPath,
            name: part,
            type: isLastPart ? file.type : 'directory',
            path: currentPath,
            children: isLastPart && file.type === 'directory' ? [] : undefined,
            file: isLastPart ? file : undefined,
            isExpanded: expandedNodes.has(currentPath),
          }

          pathMap.set(currentPath, node)
          currentLevel.push(node)
        }

        if (node.children && !isLastPart) {
          currentLevel = node.children
        }
      })
    })

    return tree
  }, [files, expandedNodes])

  // Filter tree based on search query
  const filteredTree = useMemo(() => {
    if (!searchQuery.trim()) return fileTree

    const filterNodes = (nodes: FileTreeNode[]): FileTreeNode[] => {
      return nodes.reduce<FileTreeNode[]>((acc, node) => {
        const matchesSearch = node.name.toLowerCase().includes(searchQuery.toLowerCase())
        const filteredChildren = node.children ? filterNodes(node.children) : undefined

        if (matchesSearch || (filteredChildren && filteredChildren.length > 0)) {
          acc.push({
            ...node,
            children: filteredChildren,
            isExpanded: searchQuery ? true : node.isExpanded, // Auto-expand during search
          })
        }

        return acc
      }, [])
    }

    return filterNodes(fileTree)
  }, [fileTree, searchQuery])

  // Toggle node expansion
  const toggleNode = useCallback((path: string) => {
    setExpandedNodes(prev => {
      const newSet = new Set(prev)
      if (newSet.has(path)) {
        newSet.delete(path)
      } else {
        newSet.add(path)
      }
      return newSet
    })
  }, [])

  // Handle file selection
  const handleFileSelect = useCallback((file: File) => {
    setSelectedFile(file.path)
    onFileSelect?.(file)
  }, [onFileSelect])

  // Handle context menu
  const handleContextMenu = useCallback((e: React.MouseEvent, file: File) => {
    e.preventDefault()
    setContextMenu({
      file,
      x: e.clientX,
      y: e.clientY,
    })
  }, [])

  // Close context menu
  const closeContextMenu = useCallback(() => {
    setContextMenu(null)
  }, [])

  // Start file renaming
  const startRename = useCallback((file: File) => {
    setEditingFile(file.path)
    setNewFileName(file.name)
    closeContextMenu()
  }, [closeContextMenu])

  // Finish file renaming
  const finishRename = useCallback((file: File) => {
    if (newFileName.trim() && newFileName !== file.name) {
      onFileRename?.(file, newFileName.trim())
    }
    setEditingFile(null)
    setNewFileName('')
  }, [newFileName, onFileRename])

  // Start creating new file/directory
  const startCreate = useCallback((parentPath: string, type: 'file' | 'directory') => {
    setIsCreating({ parentPath, type })
    setNewFileName('')
  }, [])

  // Finish creating new file/directory
  const finishCreate = useCallback(() => {
    if (isCreating && newFileName.trim()) {
      onFileCreate?.(isCreating.parentPath, newFileName.trim(), isCreating.type)
    }
    setIsCreating(null)
    setNewFileName('')
  }, [isCreating, newFileName, onFileCreate])

  // Render tree node
  const renderNode = useCallback((node: FileTreeNode, level: number = 0) => {
    const isDirectory = node.type === 'directory'
    const isExpanded = node.isExpanded
    const isSelected = selectedFile === node.path
    const isEditing = editingFile === node.path
    const file = node.file

    return (
      <div key={node.id}>
        {/* Node row */}
        <div
          className={cn(
            'flex items-center gap-1 py-[3px] text-sm hover:bg-white/5 cursor-pointer group transition-colors duration-100 relative',
            isSelected && 'bg-red-500/10',
          )}
          style={{ paddingLeft: `${level * 12 + 8}px`, paddingRight: '8px' }}
          onClick={() => {
            if (isDirectory) {
              toggleNode(node.path)
            } else if (file) {
              handleFileSelect(file)
            }
          }}
          onContextMenu={file ? (e) => handleContextMenu(e, file) : undefined}
        >
          {/* Selected accent bar */}
          {isSelected && (
            <div className="absolute left-0 top-0 bottom-0 w-0.5 bg-red-400 rounded-r" />
          )}

          {/* Expand/collapse icon for directories */}
          {isDirectory ? (
            <button
              className="flex items-center justify-center w-4 h-4 shrink-0 hover:bg-white/10 rounded transition-colors"
              onClick={(e) => {
                e.stopPropagation()
                toggleNode(node.path)
              }}
              aria-label={isExpanded ? 'Collapse folder' : 'Expand folder'}
            >
              {isExpanded ? (
                <ChevronDown size={11} className="text-gray-500" />
              ) : (
                <ChevronRight size={11} className="text-gray-500" />
              )}
            </button>
          ) : (
            <div className="w-4 shrink-0" />
          )}

          {/* File/directory icon */}
          <div className="flex items-center justify-center w-4 h-4 shrink-0">
            {isDirectory ? (
              isExpanded ? (
                <FolderOpen size={13} className="text-sky-400/80" />
              ) : (
                <Folder size={13} className="text-sky-400/70" />
              )
            ) : (
              <span className="text-[12px] leading-none">{getFileIcon(node.name)}</span>
            )}
          </div>

          {/* File name */}
          <div className="flex-1 min-w-0 ml-1">
            {isEditing ? (
              <Input
                value={newFileName}
                onChange={(e) => setNewFileName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    finishRename(file!)
                  } else if (e.key === 'Escape') {
                    setEditingFile(null)
                  }
                }}
                onBlur={() => finishRename(file!)}
                size="sm"
                className="h-6 py-0 text-xs"
                autoFocus
              />
            ) : (
              <span className={cn(
                'truncate block text-[13px]',
                isSelected ? 'text-white font-medium' : 'text-gray-300 group-hover:text-gray-100'
              )}>
                {node.name}
              </span>
            )}
          </div>

          {/* File badges */}
          {file && (
            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
              {file.is_locked && (
                <Lock size={12} className="text-yellow-400" />
              )}
              {showActions && (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    handleContextMenu(e, file)
                  }}
                  className="p-0.5 hover:bg-gray-700 rounded transition-colors"
                >
                  <MoreVertical size={12} className="text-gray-400" />
                </button>
              )}
            </div>
          )}
        </div>

        {/* Child nodes */}
        {isDirectory && isExpanded && node.children && (
          <div>
            {node.children.map(child => renderNode(child, level + 1))}
          </div>
        )}

        {/* New file/directory input */}
        {isCreating && isCreating.parentPath === node.path && (
          <div
            className="flex items-center gap-1 px-2 py-1 text-sm bg-gray-800/30"
            style={{ paddingLeft: `${(level + 1) * 16 + 8}px` }}
          >
            <div className="w-4 h-4" />
            <div className="flex items-center justify-center w-4 h-4 text-gray-400">
              {isCreating.type === 'directory' ? (
                <Folder size={14} className="text-cyan-400" />
              ) : (
                <FileIcon size={14} className="text-gray-400" />
              )}
            </div>
            <Input
              value={newFileName}
              onChange={(e) => setNewFileName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  finishCreate()
                } else if (e.key === 'Escape') {
                  setIsCreating(null)
                }
              }}
              onBlur={finishCreate}
              placeholder={`New ${isCreating.type} name`}
              size="sm"
              className="h-6 py-0 text-xs flex-1"
              autoFocus
            />
          </div>
        )}
      </div>
    )
  }, [
    selectedFile,
    editingFile,
    newFileName,
    isCreating,
    handleFileSelect,
    handleContextMenu,
    toggleNode,
    finishRename,
    finishCreate,
    showActions
  ])

  // Click outside to close context menu
  React.useEffect(() => {
    const handleClickOutside = () => closeContextMenu()
    if (contextMenu) {
      document.addEventListener('click', handleClickOutside)
      return () => document.removeEventListener('click', handleClickOutside)
    }
  }, [contextMenu, closeContextMenu])

  return (
    <Card variant="cyberpunk" padding="none" className={cn('h-full flex flex-col', className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700/60">
        <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-2">
          <Folder size={13} className="text-gray-500" />
          Explorer
        </h3>

        {showActions && (
          <div className="flex items-center gap-0.5">
            <button
              onClick={() => startCreate('/', 'file')}
              title="New File"
              aria-label="New File"
              className="p-1.5 rounded hover:bg-white/10 text-gray-500 hover:text-gray-300 transition-colors"
            >
              <Plus size={13} />
            </button>
            <button
              onClick={() => startCreate('/', 'directory')}
              title="New Folder"
              aria-label="New Folder"
              className="p-1.5 rounded hover:bg-white/10 text-gray-500 hover:text-gray-300 transition-colors"
            >
              <Folder size={13} />
            </button>
            <button
              onClick={() => currentProject && refreshFiles(currentProject.id)}
              title="Refresh"
              aria-label="Refresh files"
              className="p-1.5 rounded hover:bg-white/10 text-gray-500 hover:text-gray-300 transition-colors"
            >
              <RefreshCw size={13} />
            </button>
          </div>
        )}
      </div>

      {/* Search */}
      {showSearch && (
        <div className="px-2 py-2 border-b border-gray-700/50">
          <div className="relative">
            <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none" />
            <input
              type="text"
              placeholder="Search files..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-gray-800/60 border border-gray-700/50 rounded-md pl-7 pr-3 py-1.5 text-xs text-gray-300 placeholder-gray-600 focus:outline-none focus:border-gray-600 focus:bg-gray-800 transition-colors"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 transition-colors"
                aria-label="Clear search"
              >
                <X size={11} />
              </button>
            )}
          </div>
        </div>
      )}

      {/* File tree */}
      <div className="flex-1 overflow-auto py-1">
        {isLoading ? (
          <div className="flex items-center justify-center p-8">
            <Loading variant="dots" color="cyberpunk" text="Loading files..." />
          </div>
        ) : filteredTree.length > 0 ? (
          <div>
            {filteredTree.map(node => renderNode(node))}
          </div>
        ) : searchQuery ? (
          <div className="flex flex-col items-center justify-center p-8 text-center">
            <Search className="w-7 h-7 text-gray-700 mb-2" />
            <p className="text-xs text-gray-500">No files matching<br /><span className="text-gray-400">"{searchQuery}"</span></p>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center p-8 text-center">
            <FileIcon className="w-7 h-7 text-gray-700 mb-2" />
            <p className="text-xs text-gray-500 mb-4">No files in this project</p>
            <button
              onClick={() => startCreate('/', 'file')}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-gray-300 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors border border-gray-700"
            >
              <Plus size={12} />
              Create first file
            </button>
          </div>
        )}
      </div>

      {/* Context menu */}
      {contextMenu && (
        <>
          <div className="fixed inset-0 z-40" onClick={closeContextMenu} />
          <div
            className="fixed bg-gray-900/95 backdrop-blur-sm border border-gray-700/80 rounded-xl shadow-2xl shadow-black/60 z-50 py-1.5 min-w-[160px] overflow-hidden"
            style={{
              left: Math.min(contextMenu.x, window.innerWidth - 176),
              top: Math.min(contextMenu.y, window.innerHeight - 200),
            }}
          >
            <button
              className="flex items-center gap-2.5 w-full px-3 py-2 text-xs text-gray-300 hover:bg-white/5 transition-colors"
              onClick={() => startRename(contextMenu.file)}
            >
              <Edit3 size={12} className="text-gray-500" />
              Rename
            </button>

            <button
              className="flex items-center gap-2.5 w-full px-3 py-2 text-xs text-gray-300 hover:bg-white/5 transition-colors"
              onClick={() => {
                navigator.clipboard.writeText(contextMenu.file.path)
                closeContextMenu()
              }}
            >
              <Copy size={12} className="text-gray-500" />
              Copy Path
            </button>

            {contextMenu.file.type === 'directory' && (
              <>
                <div className="border-t border-gray-800 my-1" />
                <button
                  className="flex items-center gap-2.5 w-full px-3 py-2 text-xs text-gray-300 hover:bg-white/5 transition-colors"
                  onClick={() => {
                    startCreate(contextMenu.file.path, 'file')
                    closeContextMenu()
                  }}
                >
                  <Plus size={12} className="text-gray-500" />
                  New File
                </button>

                <button
                  className="flex items-center gap-2.5 w-full px-3 py-2 text-xs text-gray-300 hover:bg-white/5 transition-colors"
                  onClick={() => {
                    startCreate(contextMenu.file.path, 'directory')
                    closeContextMenu()
                  }}
                >
                  <Folder size={12} className="text-gray-500" />
                  New Folder
                </button>
              </>
            )}

            <div className="border-t border-gray-800 my-1" />

            <button
              className="flex items-center gap-2.5 w-full px-3 py-2 text-xs text-red-400 hover:bg-red-500/10 transition-colors"
              onClick={() => {
                onFileDelete?.(contextMenu.file)
                closeContextMenu()
              }}
            >
              <Trash2 size={12} />
              Delete
            </button>
          </div>
        </>
      )}
    </Card>
  )
}

export default FileTree
