// APEX.BUILD Terminal Types
// Type definitions for the full-featured terminal system

export interface TerminalTheme {
  name: string;
  foreground: string;
  background: string;
  cursor: string;
  cursorAccent: string;
  selection: string;
  black: string;
  red: string;
  green: string;
  yellow: string;
  blue: string;
  magenta: string;
  cyan: string;
  white: string;
  brightBlack: string;
  brightRed: string;
  brightGreen: string;
  brightYellow: string;
  brightBlue: string;
  brightMagenta: string;
  brightCyan: string;
  brightWhite: string;
}

export interface TerminalSession {
  id: string;
  name: string;
  projectId?: number;
  workDir: string;
  shell: string;
  status: 'connecting' | 'connected' | 'disconnected' | 'error';
  createdAt: string;
  lastActive: string;
  rows: number;
  cols: number;
  pid?: number;
}

export interface TerminalTab {
  id: string;
  sessionId: string;
  name: string;
  icon?: string;
  isActive: boolean;
  hasNotification: boolean;
  isPinned: boolean;
}

export interface TerminalPane {
  id: string;
  tabId: string;
  sessionId: string;
  direction: 'horizontal' | 'vertical';
  size: number; // percentage
  children?: TerminalPane[];
}

export interface TerminalMessage {
  type: 'input' | 'output' | 'resize' | 'signal' | 'ping' | 'pong' | 'error' | 'exit';
  data?: string;
  rows?: number;
  cols?: number;
  signal?: string;
}

export interface TerminalProcess {
  pid: number;
  name: string;
  command: string;
  cpu: number;
  memory: number;
  status: 'running' | 'sleeping' | 'stopped' | 'zombie';
  startedAt: string;
}

export interface TerminalHistoryEntry {
  command: string;
  timestamp: string;
  exitCode?: number;
  duration?: number;
}

export interface TerminalSettings {
  fontFamily: string;
  fontSize: number;
  fontWeight: string;
  lineHeight: number;
  cursorStyle: 'block' | 'underline' | 'bar';
  cursorBlink: boolean;
  scrollback: number;
  copyOnSelect: boolean;
  rightClickSelectsWord: boolean;
  bellStyle: 'none' | 'sound' | 'visual' | 'both';
  allowTransparency: boolean;
  macOptionIsMeta: boolean;
  macOptionClickForcesSelection: boolean;
  theme: string;
}

export interface TerminalShortcut {
  key: string;
  modifiers: ('ctrl' | 'alt' | 'shift' | 'meta')[];
  action: string;
  description: string;
}

export const DEFAULT_TERMINAL_SETTINGS: TerminalSettings = {
  fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Menlo, Monaco, "Courier New", monospace',
  fontSize: 14,
  fontWeight: 'normal',
  lineHeight: 1.2,
  cursorStyle: 'block',
  cursorBlink: true,
  scrollback: 10000,
  copyOnSelect: true,
  rightClickSelectsWord: true,
  bellStyle: 'visual',
  allowTransparency: true,
  macOptionIsMeta: true,
  macOptionClickForcesSelection: true,
  theme: 'cyberpunk',
};

export const TERMINAL_SHORTCUTS: TerminalShortcut[] = [
  { key: 't', modifiers: ['ctrl', 'shift'], action: 'newTab', description: 'New Terminal Tab' },
  { key: 'w', modifiers: ['ctrl', 'shift'], action: 'closeTab', description: 'Close Tab' },
  { key: 'd', modifiers: ['ctrl', 'shift'], action: 'splitHorizontal', description: 'Split Horizontal' },
  { key: 'e', modifiers: ['ctrl', 'shift'], action: 'splitVertical', description: 'Split Vertical' },
  { key: 'Tab', modifiers: ['ctrl'], action: 'nextTab', description: 'Next Tab' },
  { key: 'Tab', modifiers: ['ctrl', 'shift'], action: 'prevTab', description: 'Previous Tab' },
  { key: 'c', modifiers: ['ctrl', 'shift'], action: 'copy', description: 'Copy Selection' },
  { key: 'v', modifiers: ['ctrl', 'shift'], action: 'paste', description: 'Paste' },
  { key: 'f', modifiers: ['ctrl', 'shift'], action: 'search', description: 'Find in Terminal' },
  { key: 'k', modifiers: ['ctrl'], action: 'clear', description: 'Clear Terminal' },
  { key: 'ArrowUp', modifiers: ['alt'], action: 'scrollUp', description: 'Scroll Up' },
  { key: 'ArrowDown', modifiers: ['alt'], action: 'scrollDown', description: 'Scroll Down' },
  { key: 'Home', modifiers: ['ctrl'], action: 'scrollToTop', description: 'Scroll to Top' },
  { key: 'End', modifiers: ['ctrl'], action: 'scrollToBottom', description: 'Scroll to Bottom' },
  { key: '=', modifiers: ['ctrl'], action: 'zoomIn', description: 'Increase Font Size' },
  { key: '-', modifiers: ['ctrl'], action: 'zoomOut', description: 'Decrease Font Size' },
  { key: '0', modifiers: ['ctrl'], action: 'resetZoom', description: 'Reset Font Size' },
];

// Completion types for tab completion
export interface CompletionItem {
  value: string;
  type: 'command' | 'file' | 'directory' | 'argument' | 'env' | 'alias';
  description?: string;
  icon?: string;
}

export interface CompletionResult {
  items: CompletionItem[];
  prefix: string;
  position: number;
}
