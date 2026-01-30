// APEX.BUILD Terminal System
// Full xterm.js integration with PTY backend support

export { XTerminal } from './XTerminal';
export type { XTerminalRef, XTerminalProps } from './XTerminal';

export { TerminalManager } from './TerminalManager';

export { TerminalService } from './TerminalService';
export type { TerminalServiceCallbacks } from './TerminalService';

export {
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
  DEFAULT_TERMINAL_SETTINGS,
  TERMINAL_SHORTCUTS,
} from './types';

export { terminalThemes, getTerminalTheme, getXtermTheme } from './themes';
