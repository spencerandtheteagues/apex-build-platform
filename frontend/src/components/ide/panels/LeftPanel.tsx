// APEX.BUILD IDE Left Panel
// File explorer, search, and git panels

import React, { memo } from 'react'
import { cn } from '@/lib/utils'
import { Button, Card } from '@/components/ui'
import { FileTree } from '@/components/explorer/FileTree'
import { Folder, Search, GitBranch } from 'lucide-react'
import type { File as FileType } from '@/types'

type ActiveLeftTab = 'explorer' | 'search' | 'git'
type PanelState = 'collapsed' | 'normal' | 'expanded'

interface LeftPanelProps {
  projectId?: number
  activeTab: ActiveLeftTab
  panelState: PanelState
  onTabChange: (tab: ActiveLeftTab) => void
  onFileSelect: (file: FileType) => void
  onFileCreate: (parentPath: string, name: string, type: 'file' | 'directory') => Promise<void>
  onFileDelete: (file: FileType) => Promise<void>
  onFileRename: (file: FileType, newName: string) => Promise<void>
}

// Search Panel Component
const SearchPanel = memo(() => (
  <Card variant="cyberpunk" padding="md" className="h-full border-0">
    <div className="text-center text-gray-400">
      <Search className="w-8 h-8 mx-auto mb-2" />
      <p className="text-sm">Global search coming soon</p>
    </div>
  </Card>
))
SearchPanel.displayName = 'SearchPanel'

// Git Panel Component
const GitPanel = memo(() => (
  <Card variant="cyberpunk" padding="md" className="h-full border-0">
    <div className="text-center text-gray-400">
      <GitBranch className="w-8 h-8 mx-auto mb-2" />
      <p className="text-sm">Git integration coming soon</p>
    </div>
  </Card>
))
GitPanel.displayName = 'GitPanel'

// Panel Tab Button
interface TabButtonProps {
  active: boolean
  collapsed: boolean
  icon: React.ReactNode
  label: string
  onClick: () => void
}

const TabButton = memo<TabButtonProps>(({ active, collapsed, icon, label, onClick }) => (
  <Button
    size="xs"
    variant={active ? 'primary' : 'ghost'}
    onClick={onClick}
    icon={icon}
    className="rounded-none border-0"
  >
    {!collapsed && label}
  </Button>
))
TabButton.displayName = 'TabButton'

export const LeftPanel = memo<LeftPanelProps>(({
  projectId,
  activeTab,
  panelState,
  onTabChange,
  onFileSelect,
  onFileCreate,
  onFileDelete,
  onFileRename,
}) => {
  const isCollapsed = panelState === 'collapsed'

  const renderContent = () => {
    switch (activeTab) {
      case 'explorer':
        return (
          <FileTree
            projectId={projectId}
            onFileSelect={onFileSelect}
            onFileCreate={onFileCreate}
            onFileDelete={onFileDelete}
            onFileRename={onFileRename}
            className="h-full border-0"
          />
        )
      case 'search':
        return <SearchPanel />
      case 'git':
        return <GitPanel />
      default:
        return null
    }
  }

  return (
    <div className={cn(
      'bg-gray-900/80 border-r border-gray-800 flex flex-col transition-all duration-300',
      panelState === 'collapsed' && 'w-12',
      panelState === 'normal' && 'w-80',
      panelState === 'expanded' && 'w-96'
    )}>
      {/* Sidebar tabs */}
      <div className="h-10 bg-gray-800/50 border-b border-gray-700/50 flex items-center">
        <TabButton
          active={activeTab === 'explorer'}
          collapsed={isCollapsed}
          icon={<Folder size={14} />}
          label="Explorer"
          onClick={() => onTabChange('explorer')}
        />
        <TabButton
          active={activeTab === 'search'}
          collapsed={isCollapsed}
          icon={<Search size={14} />}
          label="Search"
          onClick={() => onTabChange('search')}
        />
        <TabButton
          active={activeTab === 'git'}
          collapsed={isCollapsed}
          icon={<GitBranch size={14} />}
          label="Git"
          onClick={() => onTabChange('git')}
        />
      </div>

      {/* Sidebar content */}
      <div className="flex-1 overflow-hidden">
        {!isCollapsed && renderContent()}
      </div>
    </div>
  )
})

LeftPanel.displayName = 'LeftPanel'

export default LeftPanel
