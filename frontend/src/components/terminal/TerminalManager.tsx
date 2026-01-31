// APEX.BUILD Terminal Manager Component
// Multi-tab terminal with shell selection and PTY integration

import React, { useState, useRef, useCallback, useEffect } from 'react';
import {
  Plus,
  X,
  Terminal as TerminalIcon,
  ChevronDown,
  Pin,
  Settings,
  Maximize2,
  Minimize2,
  Split,
  Copy,
  Trash2,
} from 'lucide-react';
import { XTerminal, XTerminalRef } from './XTerminal';
import { TerminalService } from './TerminalService';
import { TerminalSession, TerminalTab, DEFAULT_TERMINAL_SETTINGS } from './types';
import { apiService, AvailableShell } from '@/services/api';
import { cn } from '@/lib/utils';

// Using AvailableShell type from API service
type Shell = AvailableShell;

interface TerminalManagerProps {
  projectId?: number;
  workDir?: string;
  theme?: string;
  className?: string;
  onTabChange?: (tabId: string) => void;
}

export const TerminalManager: React.FC<TerminalManagerProps> = ({
  projectId,
  workDir,
  theme = 'cyberpunk',
  className,
  onTabChange,
}) => {
  const [tabs, setTabs] = useState<TerminalTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [shells, setShells] = useState<Shell[]>([]);
  const [isShellMenuOpen, setIsShellMenuOpen] = useState(false);
  const [isMaximized, setIsMaximized] = useState(false);
  const [isSplit, setIsSplit] = useState(false);
  const [splitTabId, setSplitTabId] = useState<string | null>(null);

  const terminalRefs = useRef<Map<string, XTerminalRef>>(new Map());
  const containerRef = useRef<HTMLDivElement>(null);

  // Fetch available shells on mount using API service
  useEffect(() => {
    const fetchShells = async () => {
      try {
        const availableShells = await apiService.listAvailableShells();
        if (availableShells.length > 0) {
          setShells(availableShells);
        } else {
          // Default shells if none returned
          setShells([
            { name: 'bash', path: '/bin/bash' },
            { name: 'zsh', path: '/bin/zsh' },
            { name: 'sh', path: '/bin/sh' },
          ]);
        }
      } catch (err) {
        // Default shells if API fails
        setShells([
          { name: 'bash', path: '/bin/bash' },
          { name: 'zsh', path: '/bin/zsh' },
          { name: 'sh', path: '/bin/sh' },
        ]);
      }
    };

    fetchShells();
  }, []);

  // Create initial terminal on mount
  useEffect(() => {
    if (tabs.length === 0) {
      createNewTerminal();
    }
  }, []);

  // Create a new terminal tab
  const createNewTerminal = useCallback(async (shellName?: string) => {
    const tabId = `terminal-${Date.now()}`;
    const tabNumber = tabs.length + 1;

    const newTab: TerminalTab = {
      id: tabId,
      sessionId: '', // Will be set when terminal connects
      name: shellName ? `${shellName} ${tabNumber}` : `Terminal ${tabNumber}`,
      isActive: true,
      hasNotification: false,
      isPinned: false,
    };

    // Deactivate all other tabs
    setTabs(prev => [
      ...prev.map(t => ({ ...t, isActive: false })),
      newTab,
    ]);
    setActiveTabId(tabId);
    setIsShellMenuOpen(false);
    onTabChange?.(tabId);

    return tabId;
  }, [tabs.length, onTabChange]);

  // Close a terminal tab
  const closeTab = useCallback((tabId: string, e?: React.MouseEvent) => {
    e?.stopPropagation();

    const tabIndex = tabs.findIndex(t => t.id === tabId);
    const tab = tabs[tabIndex];

    // Don't close pinned tabs without confirmation
    if (tab?.isPinned) {
      if (!confirm('Close pinned terminal?')) {
        return;
      }
    }

    // Clean up terminal ref
    const termRef = terminalRefs.current.get(tabId);
    if (termRef) {
      // Disconnect the session using the API service
      const session = termRef.getSession();
      if (session?.id) {
        apiService.closeTerminalSession(session.id).catch(() => {});
      }
      terminalRefs.current.delete(tabId);
    }

    // Remove tab
    const newTabs = tabs.filter(t => t.id !== tabId);
    setTabs(newTabs);

    // If this was the active tab, activate another one
    if (activeTabId === tabId && newTabs.length > 0) {
      const newActiveIndex = Math.min(tabIndex, newTabs.length - 1);
      const newActiveTab = newTabs[newActiveIndex];
      setActiveTabId(newActiveTab.id);
      newTabs[newActiveIndex].isActive = true;
      setTabs([...newTabs]);
      onTabChange?.(newActiveTab.id);
    } else if (newTabs.length === 0) {
      setActiveTabId(null);
    }

    // Clear split if closing split tab
    if (splitTabId === tabId) {
      setIsSplit(false);
      setSplitTabId(null);
    }
  }, [tabs, activeTabId, splitTabId, onTabChange]);

  // Switch to a tab
  const switchTab = useCallback((tabId: string) => {
    setTabs(prev => prev.map(t => ({
      ...t,
      isActive: t.id === tabId,
      hasNotification: t.id === tabId ? false : t.hasNotification,
    })));
    setActiveTabId(tabId);
    onTabChange?.(tabId);

    // Focus the terminal
    setTimeout(() => {
      const termRef = terminalRefs.current.get(tabId);
      termRef?.focus();
    }, 50);
  }, [onTabChange]);

  // Toggle pin on a tab
  const togglePin = useCallback((tabId: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    setTabs(prev => prev.map(t => ({
      ...t,
      isPinned: t.id === tabId ? !t.isPinned : t.isPinned,
    })));
  }, []);

  // Handle session creation
  const handleSessionCreate = useCallback((tabId: string, session: TerminalSession) => {
    setTabs(prev => prev.map(t =>
      t.id === tabId ? { ...t, sessionId: session.id } : t
    ));
  }, []);

  // Handle title change from terminal
  const handleTitleChange = useCallback((tabId: string, title: string) => {
    setTabs(prev => prev.map(t =>
      t.id === tabId ? { ...t, name: title || t.name } : t
    ));
  }, []);

  // Store terminal ref
  const setTerminalRef = useCallback((tabId: string, ref: XTerminalRef | null) => {
    if (ref) {
      terminalRefs.current.set(tabId, ref);
    } else {
      terminalRefs.current.delete(tabId);
    }
  }, []);

  // Toggle split view
  const toggleSplit = useCallback(() => {
    if (isSplit) {
      setIsSplit(false);
      setSplitTabId(null);
    } else if (tabs.length >= 2) {
      // Find another tab to show in split
      const otherTab = tabs.find(t => t.id !== activeTabId);
      if (otherTab) {
        setIsSplit(true);
        setSplitTabId(otherTab.id);
      }
    }
  }, [isSplit, tabs, activeTabId]);

  // Duplicate terminal
  const duplicateTerminal = useCallback(async (tabId: string) => {
    const tab = tabs.find(t => t.id === tabId);
    if (tab) {
      const termRef = terminalRefs.current.get(tabId);
      const session = termRef?.getSession();
      // Create new terminal with same shell
      await createNewTerminal(session?.shell?.split('/').pop() || 'bash');
    }
  }, [tabs, createNewTerminal]);

  // Get active tab
  const activeTab = tabs.find(t => t.id === activeTabId);
  const splitTab = splitTabId ? tabs.find(t => t.id === splitTabId) : null;

  return (
    <div
      ref={containerRef}
      className={cn(
        'flex flex-col bg-[#0a0a0a] rounded-lg overflow-hidden',
        isMaximized ? 'fixed inset-0 z-50' : 'h-full',
        className
      )}
    >
      {/* Tab Bar */}
      <div className="flex items-center bg-[#1a1a2e] border-b border-cyan-900/30 h-10 min-h-[40px]">
        {/* Tabs */}
        <div className="flex-1 flex items-center overflow-x-auto scrollbar-hide">
          {tabs.map((tab) => (
            <div
              key={tab.id}
              onClick={() => switchTab(tab.id)}
              className={cn(
                'group flex items-center gap-2 px-3 py-2 cursor-pointer border-r border-cyan-900/30 min-w-[120px] max-w-[200px]',
                'transition-colors duration-150',
                tab.isActive
                  ? 'bg-[#0a0a0a] text-cyan-400'
                  : 'bg-[#1a1a2e] text-gray-400 hover:bg-[#252542] hover:text-gray-200',
                tab.hasNotification && !tab.isActive && 'text-yellow-400'
              )}
            >
              <TerminalIcon size={14} className={tab.isPinned ? 'text-cyan-400' : ''} />
              <span className="truncate text-sm flex-1">{tab.name}</span>
              {tab.isPinned && (
                <Pin size={12} className="text-cyan-400 opacity-60" />
              )}
              <button
                onClick={(e) => closeTab(tab.id, e)}
                className="opacity-0 group-hover:opacity-100 hover:text-red-400 transition-opacity"
              >
                <X size={14} />
              </button>
            </div>
          ))}
        </div>

        {/* Actions */}
        <div className="flex items-center gap-1 px-2 border-l border-cyan-900/30">
          {/* New Terminal Dropdown */}
          <div className="relative">
            <button
              onClick={() => setIsShellMenuOpen(!isShellMenuOpen)}
              className="flex items-center gap-1 px-2 py-1 text-gray-400 hover:text-cyan-400 hover:bg-cyan-900/20 rounded transition-colors"
              title="New Terminal"
            >
              <Plus size={16} />
              <ChevronDown size={12} />
            </button>

            {isShellMenuOpen && (
              <div className="absolute right-0 top-full mt-1 w-40 bg-[#1a1a2e] border border-cyan-900/50 rounded-lg shadow-xl z-50 py-1">
                <div className="px-3 py-1 text-xs text-gray-500 uppercase">Shell</div>
                {shells.map((shell) => (
                  <button
                    key={shell.path}
                    onClick={() => createNewTerminal(shell.name)}
                    className="w-full px-3 py-2 text-left text-sm text-gray-300 hover:bg-cyan-900/30 hover:text-cyan-400 flex items-center gap-2"
                  >
                    <TerminalIcon size={14} />
                    {shell.name}
                  </button>
                ))}
                <div className="border-t border-cyan-900/30 my-1" />
                <button
                  onClick={() => createNewTerminal()}
                  className="w-full px-3 py-2 text-left text-sm text-gray-300 hover:bg-cyan-900/30 hover:text-cyan-400 flex items-center gap-2"
                >
                  <Plus size={14} />
                  Default Shell
                </button>
              </div>
            )}
          </div>

          {/* Split View */}
          <button
            onClick={toggleSplit}
            disabled={tabs.length < 2}
            className={cn(
              'p-1.5 rounded transition-colors',
              isSplit
                ? 'text-cyan-400 bg-cyan-900/30'
                : 'text-gray-400 hover:text-cyan-400 hover:bg-cyan-900/20',
              tabs.length < 2 && 'opacity-50 cursor-not-allowed'
            )}
            title="Split View"
          >
            <Split size={16} />
          </button>

          {/* Maximize */}
          <button
            onClick={() => setIsMaximized(!isMaximized)}
            className="p-1.5 text-gray-400 hover:text-cyan-400 hover:bg-cyan-900/20 rounded transition-colors"
            title={isMaximized ? 'Restore' : 'Maximize'}
          >
            {isMaximized ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
          </button>
        </div>
      </div>

      {/* Terminal Content */}
      <div className={cn('flex-1 flex', isSplit ? 'flex-row' : 'flex-col')}>
        {/* Main Terminal */}
        {activeTab && (
          <div className={cn('flex-1 min-h-0', isSplit && 'border-r border-cyan-900/30')}>
            <XTerminal
              key={activeTab.id}
              ref={(ref) => setTerminalRef(activeTab.id, ref)}
              projectId={projectId}
              workDir={workDir}
              theme={theme}
              className="h-full"
              onSessionCreate={(session) => handleSessionCreate(activeTab.id, session)}
              onTitleChange={(title) => handleTitleChange(activeTab.id, title)}
            />
          </div>
        )}

        {/* Split Terminal */}
        {isSplit && splitTab && (
          <div className="flex-1 min-h-0">
            <XTerminal
              key={splitTab.id}
              ref={(ref) => setTerminalRef(splitTab.id, ref)}
              projectId={projectId}
              workDir={workDir}
              theme={theme}
              className="h-full"
              onSessionCreate={(session) => handleSessionCreate(splitTab.id, session)}
              onTitleChange={(title) => handleTitleChange(splitTab.id, title)}
            />
          </div>
        )}

        {/* Empty State */}
        {tabs.length === 0 && (
          <div className="flex-1 flex items-center justify-center text-gray-500">
            <div className="text-center">
              <TerminalIcon size={48} className="mx-auto mb-4 opacity-50" />
              <p className="mb-4">No terminals open</p>
              <button
                onClick={() => createNewTerminal()}
                className="px-4 py-2 bg-cyan-600 text-white rounded-lg hover:bg-cyan-500 transition-colors"
              >
                Create Terminal
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Status Bar */}
      <div className="flex items-center justify-between px-3 py-1 bg-[#1a1a2e] border-t border-cyan-900/30 text-xs text-gray-500">
        <div className="flex items-center gap-4">
          {activeTab && (
            <>
              <span>Session: {tabs.find(t => t.id === activeTabId)?.sessionId?.slice(0, 8) || 'connecting...'}</span>
              <span>Shell: {terminalRefs.current.get(activeTabId || '')?.getSession()?.shell || 'bash'}</span>
            </>
          )}
        </div>
        <div className="flex items-center gap-4">
          <span>{tabs.length} terminal{tabs.length !== 1 ? 's' : ''}</span>
        </div>
      </div>
    </div>
  );
};

export default TerminalManager;
