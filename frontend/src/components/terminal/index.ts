// APEX.BUILD Terminal System
// Full xterm.js integration with PTY backend support

export { XTerminal } from './XTerminal';
export type { XTerminalRef, XTerminalProps } from './XTerminal';

export { TerminalManager } from './TerminalManager';

export { TerminalService } from './TerminalService';
export type { TerminalServiceCallbacks } from './TerminalService';

export type {
  TerminalSession,
  TerminalTab,
  TerminalPane,
  TerminalMessage,
  TerminalTheme,
  TerminalSettings,
  TerminalShortcut,
  TerminalProcess,
  TerminalHistoryEntry,
  CompletionItem,
  CompletionResult,
} from './types'

export { terminalThemes, getTerminalTheme, getXtermTheme } from './themes';
