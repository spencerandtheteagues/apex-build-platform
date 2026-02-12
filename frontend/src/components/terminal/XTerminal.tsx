// APEX.BUILD XTerminal Component
// Full xterm.js integration with all addons

import React, { useEffect, useRef, useCallback, useState, forwardRef, useImperativeHandle } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { SearchAddon } from '@xterm/addon-search';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { SerializeAddon } from '@xterm/addon-serialize';
import 'xterm/css/xterm.css';

import { TerminalService } from './TerminalService';
import { TerminalSession, DEFAULT_TERMINAL_SETTINGS, TerminalSettings } from './types';
import { getXtermTheme } from './themes';
import { cn } from '@/lib/utils';

export interface XTerminalProps {
  sessionId?: string;
  projectId?: number;
  workDir?: string;
  shell?: string; // Shell to use: bash, zsh, sh, or path
  name?: string; // Terminal name/title
  theme?: string;
  settings?: Partial<TerminalSettings>;
  environment?: Record<string, string>;
  className?: string;
  onSessionCreate?: (session: TerminalSession) => void;
  onSessionEnd?: () => void;
  onTitleChange?: (title: string) => void;
  onReady?: (terminal: Terminal) => void;
}

export interface XTerminalRef {
  terminal: Terminal | null;
  fitAddon: FitAddon | null;
  searchAddon: SearchAddon | null;
  write: (data: string) => void;
  writeln: (data: string) => void;
  clear: () => void;
  focus: () => void;
  fit: () => void;
  findNext: (term: string) => boolean;
  findPrevious: (term: string) => boolean;
  serialize: () => string;
  sendInput: (data: string) => void;
  sendSignal: (signal: string) => void;
  getSession: () => TerminalSession | null;
}

export const XTerminal = forwardRef<XTerminalRef, XTerminalProps>(({
  sessionId,
  projectId,
  workDir,
  shell,
  name,
  theme = 'cyberpunk',
  settings: userSettings,
  environment,
  className,
  onSessionCreate,
  onSessionEnd,
  onTitleChange,
  onReady,
}, ref) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const serviceRef = useRef<TerminalService | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const searchAddonRef = useRef<SearchAddon | null>(null);
  const serializeAddonRef = useRef<SerializeAddon | null>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);

  const [currentSession, setCurrentSession] = useState<TerminalSession | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [connectionError, setConnectionError] = useState<string | null>(null);

  // Merge settings
  const settings = { ...DEFAULT_TERMINAL_SETTINGS, ...userSettings };

  // Initialize terminal
  useEffect(() => {
    if (!containerRef.current) return;

    // Create terminal instance
    const terminal = new Terminal({
      fontFamily: settings.fontFamily,
      fontSize: settings.fontSize,
      fontWeight: settings.fontWeight as any,
      lineHeight: settings.lineHeight,
      cursorStyle: settings.cursorStyle,
      cursorBlink: settings.cursorBlink,
      scrollback: settings.scrollback,
      allowTransparency: settings.allowTransparency,
      macOptionIsMeta: settings.macOptionIsMeta,
      macOptionClickForcesSelection: settings.macOptionClickForcesSelection,
      theme: getXtermTheme(theme),
      allowProposedApi: true,
      convertEol: true,
      scrollOnUserInput: true,
    });

    // Initialize addons
    const fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();
    const searchAddon = new SearchAddon();
    const unicode11Addon = new Unicode11Addon();
    const serializeAddon = new SerializeAddon();

    terminal.loadAddon(fitAddon);
    terminal.loadAddon(webLinksAddon);
    terminal.loadAddon(searchAddon);
    terminal.loadAddon(unicode11Addon);
    terminal.loadAddon(serializeAddon);

    // Activate unicode11
    terminal.unicode.activeVersion = '11';

    // Open terminal in container
    terminal.open(containerRef.current);

    // Initial fit
    try {
      fitAddon.fit();
    } catch (e) {
      console.warn('Initial fit failed:', e);
    }

    // Store refs
    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;
    searchAddonRef.current = searchAddon;
    serializeAddonRef.current = serializeAddon;

    // Handle copy on select
    if (settings.copyOnSelect) {
      terminal.onSelectionChange(() => {
        const selection = terminal.getSelection();
        if (selection) {
          navigator.clipboard.writeText(selection).catch(() => {});
        }
      });
    }

    // Title change
    terminal.onTitleChange((title) => {
      onTitleChange?.(title);
    });

    // Handle bell
    terminal.onBell(() => {
      if (settings.bellStyle === 'visual' || settings.bellStyle === 'both') {
        if (containerRef.current) {
          containerRef.current.classList.add('terminal-bell');
          setTimeout(() => {
            containerRef.current?.classList.remove('terminal-bell');
          }, 200);
        }
      }
    });

    // Setup resize observer
    resizeObserverRef.current = new ResizeObserver(() => {
      try {
        fitAddon.fit();
        // Notify backend of resize
        if (serviceRef.current && terminal.rows && terminal.cols) {
          serviceRef.current.resize(terminal.rows, terminal.cols);
        }
      } catch (e) {
        // Ignore resize errors during unmount
      }
    });
    resizeObserverRef.current.observe(containerRef.current);

    // Notify ready
    onReady?.(terminal);

    // Cleanup
    return () => {
      resizeObserverRef.current?.disconnect();
      terminal.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
      searchAddonRef.current = null;
      serializeAddonRef.current = null;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- terminal bootstrap is intentionally one-time per mount.

  // Update theme
  useEffect(() => {
    if (terminalRef.current) {
      terminalRef.current.options.theme = getXtermTheme(theme);
    }
  }, [theme]);

  // Initialize terminal service and connect
  useEffect(() => {
    if (!terminalRef.current) return;

    const terminal = terminalRef.current;

    // Create terminal service with callbacks
    const service = new TerminalService({
      onData: (data) => {
        terminal.write(data);
      },
      onConnect: () => {
        setIsConnected(true);
        setConnectionError(null);
        terminal.focus();
      },
      onDisconnect: () => {
        setIsConnected(false);
        terminal.writeln('\r\n\x1b[33m[Terminal Disconnected]\x1b[0m');
      },
      onError: (error) => {
        setConnectionError(error);
        terminal.writeln(`\r\n\x1b[31m[Error: ${error}]\x1b[0m`);
      },
      onExit: (message) => {
        terminal.writeln(`\r\n\x1b[33m[${message}]\x1b[0m`);
        onSessionEnd?.();
      },
    });

    serviceRef.current = service;

    // Handle terminal input
    const inputDisposable = terminal.onData((data) => {
      if (service.isConnected()) {
        service.sendInput(data);
      }
    });

    // Connect to session
    const initSession = async () => {
      try {
        if (sessionId) {
          // Connect to existing session
          await service.connect(sessionId);
          const sessionData = await service.getSession(sessionId);
          if (sessionData) {
            setCurrentSession(sessionData);
          }
        } else {
          // Create new session
          terminal.writeln('\x1b[36mCreating terminal session...\x1b[0m');
          const newSession = await service.createSession(projectId, workDir, {
            shell,
            name,
            rows: terminal.rows,
            cols: terminal.cols,
            environment,
          });
          setCurrentSession(newSession);
          onSessionCreate?.(newSession);

          // Connect to the new session
          await service.connect(newSession.id);
        }

        // Send initial resize
        if (terminal.rows && terminal.cols) {
          service.resize(terminal.rows, terminal.cols);
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to connect';
        setConnectionError(message);
        terminal.writeln(`\r\n\x1b[31mConnection failed: ${message}\x1b[0m`);
        terminal.writeln('\x1b[33mTrying local terminal emulation...\x1b[0m');
        terminal.writeln('');
        terminal.writeln('\x1b[32mapex@build\x1b[0m:\x1b[34m~\x1b[0m$ ');
      }
    };

    initSession();

    // Cleanup
    return () => {
      inputDisposable.dispose();
      service.disconnect();
      serviceRef.current = null;
    };
  }, [sessionId, projectId, workDir, shell, name, environment]); // eslint-disable-line react-hooks/exhaustive-deps -- session callbacks are intentionally bound at connect time.

  // Expose methods via ref
  useImperativeHandle(ref, () => ({
    terminal: terminalRef.current,
    fitAddon: fitAddonRef.current,
    searchAddon: searchAddonRef.current,

    write: (data: string) => {
      terminalRef.current?.write(data);
    },

    writeln: (data: string) => {
      terminalRef.current?.writeln(data);
    },

    clear: () => {
      terminalRef.current?.clear();
    },

    focus: () => {
      terminalRef.current?.focus();
    },

    fit: () => {
      try {
        fitAddonRef.current?.fit();
      } catch (e) {
        console.warn('Fit failed:', e);
      }
    },

    findNext: (term: string) => {
      return searchAddonRef.current?.findNext(term) || false;
    },

    findPrevious: (term: string) => {
      return searchAddonRef.current?.findPrevious(term) || false;
    },

    serialize: () => {
      return serializeAddonRef.current?.serialize() || '';
    },

    sendInput: (data: string) => {
      serviceRef.current?.sendInput(data);
    },

    sendSignal: (signal: string) => {
      serviceRef.current?.sendSignal(signal);
    },

    getSession: () => currentSession,
  }), [currentSession]);

  return (
    <div
      ref={containerRef}
      className={cn(
        'w-full h-full min-h-[200px] bg-black rounded overflow-hidden',
        'focus-within:ring-2 focus-within:ring-primary/50',
        className
      )}
      style={{
        padding: '4px',
      }}
    />
  );
});

XTerminal.displayName = 'XTerminal';

export default XTerminal;
