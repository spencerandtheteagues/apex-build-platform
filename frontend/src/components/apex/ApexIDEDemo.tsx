/**
 * APEX.BUILD IDE Demo
 * Interactive prototype showcasing the futuristic cyberpunk interface
 * This is what will make users abandon Replit immediately
 */

import React, { useState, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  APEXButton,
  APEXCard,
  APEXInput,
  APEXTitle,
  APEXNav,
  APEXLoading,
  APEXParticleBackground,
  APEXThemeProvider,
  useAPEXTheme,
  APEX_TOKENS
} from './apex-components';

// ===== ICONS (Using simple SVGs for demo) =====
const Icons = {
  Code: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M9.4 16.6L4.8 12l4.6-4.6L8 6l-6 6 6 6 1.4-1.4zm5.2 0L19.2 12l-4.6-4.6L16 6l6 6-6 6-1.4-1.4z"/>
    </svg>
  ),
  Terminal: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M2 12h6v2H2v-2zm18-7v14c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V5c0-1.1.9-2 2-2h14c1.1 0 2 .9 2 2zm-2 0H4v14h14V5zM6 7h2v1H6V7zm2 2H6v1h2V9zm2-2h2v1h-2V7z"/>
    </svg>
  ),
  Files: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M10 4H4c-1.11 0-2 .89-2 2v12c0 1.11.89 2 2 2h16c1.11 0 2-.89 2-2V8c0-1.11-.89-2-2-2h-8l-2-2z"/>
    </svg>
  ),
  Settings: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M19.14,12.94c0.04-0.3,0.06-0.61,0.06-0.94c0-0.32-0.02-0.64-0.07-0.94l2.03-1.58c0.18-0.14,0.23-0.41,0.12-0.61 l-1.92-3.32c-0.12-0.22-0.37-0.29-0.59-0.22l-2.39,0.96c-0.5-0.38-1.03-0.7-1.62-0.94L14.4,2.81c-0.04-0.24-0.24-0.41-0.48-0.41 h-3.84c-0.24,0-0.43,0.17-0.47,0.41L9.25,5.35C8.66,5.59,8.12,5.92,7.63,6.29L5.24,5.33c-0.22-0.08-0.47,0-0.59,0.22L2.74,8.87 C2.62,9.08,2.66,9.34,2.86,9.48l2.03,1.58C4.84,11.36,4.82,11.69,4.82,12s0.02,0.64,0.07,0.94l-2.03,1.58 c-0.18,0.14-0.23,0.41-0.12,0.61l1.92,3.32c0.12,0.22,0.37,0.29,0.59,0.22l2.39-0.96c0.5,0.38,1.03,0.7,1.62,0.94l0.36,2.54 c0.05,0.24,0.24,0.41,0.48,0.41h3.84c0.24,0,0.44-0.17,0.47-0.41l0.36-2.54c0.59-0.24,1.13-0.56,1.62-0.94l2.39,0.96 c0.22,0.08,0.47,0,0.59-0.22l1.92-3.32c0.12-0.22,0.07-0.47-0.12-0.61L19.14,12.94z M12,15.6c-1.98,0-3.6-1.62-3.6-3.6 s1.62-3.6,3.6-3.6s3.6,1.62,3.6,3.6S13.98,15.6,12,15.6z"/>
    </svg>
  ),
  Play: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M8 5v14l11-7z"/>
    </svg>
  ),
  Stop: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M6 6h12v12H6z"/>
    </svg>
  ),
  AI: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
    </svg>
  ),
  Deploy: () => (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z"/>
    </svg>
  )
};

// ===== MOCK DATA =====
const MOCK_FILES = [
  { name: 'src', type: 'folder', children: [
    { name: 'components', type: 'folder', children: [
      { name: 'App.tsx', type: 'file', language: 'typescript' },
      { name: 'Button.tsx', type: 'file', language: 'typescript' },
      { name: 'Card.tsx', type: 'file', language: 'typescript' },
    ]},
    { name: 'hooks', type: 'folder', children: [
      { name: 'useAuth.ts', type: 'file', language: 'typescript' },
      { name: 'useAPI.ts', type: 'file', language: 'typescript' },
    ]},
    { name: 'utils', type: 'folder', children: [
      { name: 'helpers.ts', type: 'file', language: 'typescript' },
    ]},
    { name: 'main.tsx', type: 'file', language: 'typescript' },
  ]},
  { name: 'public', type: 'folder', children: [
    { name: 'index.html', type: 'file', language: 'html' },
    { name: 'favicon.ico', type: 'file', language: 'image' },
  ]},
  { name: 'package.json', type: 'file', language: 'json' },
  { name: 'README.md', type: 'file', language: 'markdown' },
  { name: 'tsconfig.json', type: 'file', language: 'json' },
];

const SAMPLE_CODE = `import React, { useState } from 'react';
import { motion } from 'framer-motion';

interface ButtonProps {
  variant?: 'primary' | 'secondary';
  children: React.ReactNode;
  onClick?: () => void;
}

export const Button: React.FC<ButtonProps> = ({
  variant = 'primary',
  children,
  onClick
}) => {
  const [isHovered, setIsHovered] = useState(false);

  return (
    <motion.button
      className={\`btn btn-\${variant}\`}
      whileHover={{ scale: 1.05, y: -2 }}
      whileTap={{ scale: 0.98 }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onClick={onClick}
      style={{
        background: isHovered
          ? 'linear-gradient(135deg, #00f5ff, #ff0080)'
          : 'linear-gradient(135deg, #1a1a2e, #16213e)',
        border: '1px solid #00f5ff',
        borderRadius: '8px',
        padding: '12px 24px',
        color: '#ffffff',
        fontWeight: 600,
        cursor: 'pointer',
        transition: 'all 0.3s ease',
        boxShadow: isHovered
          ? '0 0 20px rgba(0, 245, 255, 0.5)'
          : 'none'
      }}
    >
      {children}
    </motion.button>
  );
};`;

// ===== FILE EXPLORER COMPONENT =====
const FileExplorer: React.FC<{ files: any[], onFileSelect: (file: any) => void }> = ({
  files,
  onFileSelect
}) => {
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set(['src']));

  const toggleFolder = (path: string) => {
    const newExpanded = new Set(expandedFolders);
    if (newExpanded.has(path)) {
      newExpanded.delete(path);
    } else {
      newExpanded.add(path);
    }
    setExpandedFolders(newExpanded);
  };

  const renderFile = (file: any, path: string = '', depth: number = 0) => {
    const currentPath = path ? `${path}/${file.name}` : file.name;
    const isExpanded = expandedFolders.has(currentPath);

    return (
      <div key={currentPath} style={{ marginLeft: `${depth * 16}px` }}>
        <motion.div
          className="file-item"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '6px 8px',
            borderRadius: '4px',
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            fontSize: '0.875rem',
            fontFamily: 'Space Grotesk, sans-serif',
          }}
          whileHover={{
            background: 'rgba(0, 245, 255, 0.1)',
            boxShadow: '0 0 10px rgba(0, 245, 255, 0.2)',
          }}
          onClick={() => {
            if (file.type === 'folder') {
              toggleFolder(currentPath);
            } else {
              onFileSelect(file);
            }
          }}
        >
          {file.type === 'folder' ? (
            <motion.div
              animate={{ rotate: isExpanded ? 90 : 0 }}
              style={{ fontSize: '12px', color: APEX_TOKENS.colors.accent }}
            >
              ▶
            </motion.div>
          ) : (
            <div style={{ width: '12px' }} />
          )}

          <div style={{
            color: file.type === 'folder' ? APEX_TOKENS.colors.primary : APEX_TOKENS.colors.text
          }}>
            {file.name}
          </div>
        </motion.div>

        <AnimatePresence>
          {file.type === 'folder' && isExpanded && file.children && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.2 }}
            >
              {file.children.map((child: any) => renderFile(child, currentPath, depth + 1))}
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    );
  };

  return (
    <div style={{
      padding: '16px',
      height: '100%',
      overflowY: 'auto',
      fontSize: '14px',
    }}>
      {files.map((file) => renderFile(file))}
    </div>
  );
};

// ===== CODE EDITOR COMPONENT =====
const CodeEditor: React.FC<{
  code: string;
  onChange: (code: string) => void;
  language: string
}> = ({ code, onChange, language }) => {
  const [isAIAssisting, setIsAIAssisting] = useState(false);

  const simulateAIAssistance = () => {
    setIsAIAssisting(true);
    setTimeout(() => {
      const aiSuggestion = `\n  // AI Suggestion: Add error handling\n  try {\n    ${code.split('\n').slice(-3, -1).join('\n    ')}\n  } catch (error) {\n    console.error('Error:', error);\n  }`;
      onChange(code + aiSuggestion);
      setIsAIAssisting(false);
    }, 2000);
  };

  return (
    <div style={{
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      background: APEX_TOKENS.colors.background,
      position: 'relative',
    }}>
      {/* Editor Header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '12px 16px',
        background: 'rgba(26, 26, 46, 0.9)',
        borderBottom: `1px solid ${APEX_TOKENS.colors.primary}33`,
      }}>
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
          color: APEX_TOKENS.colors.text,
          fontSize: '0.875rem',
          fontFamily: 'Space Grotesk, sans-serif',
        }}>
          <span>App.tsx</span>
          <div style={{
            background: APEX_TOKENS.colors.accent,
            color: APEX_TOKENS.colors.background,
            padding: '2px 8px',
            borderRadius: '4px',
            fontSize: '0.75rem',
            fontWeight: 600,
          }}>
            {language.toUpperCase()}
          </div>
        </div>

        <div style={{ display: 'flex', gap: '8px' }}>
          <APEXButton
            variant="ghost"
            size="sm"
            onClick={simulateAIAssistance}
            disabled={isAIAssisting}
            icon={<Icons.AI />}
          >
            {isAIAssisting ? 'AI Working...' : 'AI Assist'}
          </APEXButton>
          <APEXButton variant="primary" size="sm" icon={<Icons.Play />}>
            Run
          </APEXButton>
        </div>
      </div>

      {/* Code Area */}
      <div style={{
        flex: 1,
        display: 'flex',
        position: 'relative',
        overflow: 'hidden',
      }}>
        {/* Line Numbers */}
        <div style={{
          width: '60px',
          background: 'rgba(10, 10, 10, 0.8)',
          borderRight: `1px solid ${APEX_TOKENS.colors.primary}33`,
          padding: '16px 8px',
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: '14px',
          lineHeight: '1.5',
          color: APEX_TOKENS.colors.primary,
          textAlign: 'right',
        }}>
          {code.split('\n').map((_, index) => (
            <div key={index} style={{ opacity: 0.7 }}>
              {index + 1}
            </div>
          ))}
        </div>

        {/* Code Input */}
        <textarea
          value={code}
          onChange={(e) => onChange(e.target.value)}
          style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            outline: 'none',
            resize: 'none',
            padding: '16px',
            fontFamily: 'JetBrains Mono, monospace',
            fontSize: '14px',
            lineHeight: '1.5',
            color: APEX_TOKENS.colors.text,
            whiteSpace: 'pre',
            overflowWrap: 'normal',
            overflowX: 'auto',
            tabSize: 2,
          }}
          spellCheck={false}
        />

        {/* AI Loading Overlay */}
        <AnimatePresence>
          {isAIAssisting && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              style={{
                position: 'absolute',
                top: 0,
                right: 0,
                bottom: 0,
                left: 0,
                background: 'rgba(0, 0, 0, 0.8)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexDirection: 'column',
                gap: '16px',
              }}
            >
              <APEXLoading variant="digital" size="lg" />
              <div style={{
                color: APEX_TOKENS.colors.primary,
                fontSize: '1.125rem',
                fontWeight: 600,
                fontFamily: 'Space Grotesk, sans-serif',
              }}>
                AI is analyzing your code...
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
};

// ===== MAIN IDE COMPONENT =====
export const APEXIDEDemo: React.FC = () => {
  const [selectedFile, setSelectedFile] = useState(MOCK_FILES[0].children?.[3] || MOCK_FILES[0]); // main.tsx
  const [code, setCode] = useState(SAMPLE_CODE);
  const [isRunning, setIsRunning] = useState(false);
  const [sidebarWidth, setSidebarWidth] = useState(300);
  const { theme, setTheme } = useAPEXTheme();

  const navItems = [
    { label: 'Explorer', icon: <Icons.Files />, active: true },
    { label: 'Search', icon: <Icons.Code /> },
    { label: 'Git', icon: <Icons.Terminal /> },
    { label: 'Debug', icon: <Icons.Play /> },
    { label: 'Settings', icon: <Icons.Settings /> },
  ];

  const themes = [
    { id: 'cyberpunk', name: 'Cyberpunk', color: '#00f5ff' },
    { id: 'matrix', name: 'Matrix', color: '#00ff41' },
    { id: 'synthwave', name: 'Synthwave', color: '#ff006e' },
    { id: 'neonCity', name: 'Neon City', color: '#00d4ff' },
  ];

  const simulateRun = () => {
    setIsRunning(true);
    setTimeout(() => {
      setIsRunning(false);
    }, 3000);
  };

  return (
    <div style={{
      height: '100vh',
      background: APEX_TOKENS.colors.background,
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      fontFamily: 'Space Grotesk, sans-serif',
    }}>
      {/* Particle Background */}
      <APEXParticleBackground density={150} />

      {/* Top Header */}
      <div style={{
        height: '60px',
        background: 'rgba(26, 26, 46, 0.95)',
        backdropFilter: 'blur(10px)',
        borderBottom: `1px solid ${APEX_TOKENS.colors.primary}33`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 24px',
        zIndex: 100,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
          <APEXTitle level={3} glowColor={APEX_TOKENS.colors.primary}>
            APEX.BUILD
          </APEXTitle>
          <div style={{
            fontSize: '0.875rem',
            color: APEX_TOKENS.colors.accent,
            fontWeight: 600,
          }}>
            The Future of Cloud Development
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          {/* Theme Selector */}
          <select
            value={theme}
            onChange={(e) => setTheme(e.target.value as any)}
            style={{
              background: 'rgba(26, 26, 46, 0.9)',
              border: `1px solid ${APEX_TOKENS.colors.primary}66`,
              borderRadius: '6px',
              color: APEX_TOKENS.colors.text,
              padding: '6px 12px',
              fontSize: '0.875rem',
              fontFamily: 'Space Grotesk, sans-serif',
            }}
          >
            {themes.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>

          <APEXButton
            variant="holographic"
            size="sm"
            onClick={simulateRun}
            disabled={isRunning}
            icon={isRunning ? <Icons.Stop /> : <Icons.Deploy />}
          >
            {isRunning ? 'Deploying...' : 'Deploy'}
          </APEXButton>
        </div>
      </div>

      {/* Main Content */}
      <div style={{
        flex: 1,
        display: 'flex',
        overflow: 'hidden',
      }}>
        {/* Sidebar */}
        <div style={{
          width: `${sidebarWidth}px`,
          display: 'flex',
          borderRight: `1px solid ${APEX_TOKENS.colors.primary}33`,
        }}>
          {/* Navigation */}
          <div style={{
            width: '60px',
            background: 'rgba(16, 16, 30, 0.95)',
            borderRight: `1px solid ${APEX_TOKENS.colors.primary}33`,
          }}>
            <APEXNav items={navItems} orientation="vertical" />
          </div>

          {/* File Explorer */}
          <APEXCard
            variant="glass"
            style={{
              flex: 1,
              borderRadius: 0,
              height: '100%',
              padding: 0,
            }}
          >
            <div style={{
              padding: '16px',
              borderBottom: `1px solid ${APEX_TOKENS.colors.primary}33`,
              fontSize: '0.875rem',
              fontWeight: 600,
              color: APEX_TOKENS.colors.primary,
            }}>
              PROJECT EXPLORER
            </div>
            <FileExplorer
              files={MOCK_FILES}
              onFileSelect={setSelectedFile}
            />
          </APEXCard>
        </div>

        {/* Main Editor Area */}
        <div style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
        }}>
          <APEXCard
            variant="glass"
            style={{
              flex: 1,
              borderRadius: 0,
              padding: 0,
              height: '100%',
            }}
          >
            <CodeEditor
              code={code}
              onChange={setCode}
              language="typescript"
            />
          </APEXCard>

          {/* Bottom Terminal/Output */}
          <APEXCard
            variant="neon"
            style={{
              height: '200px',
              borderRadius: 0,
              borderTop: `1px solid ${APEX_TOKENS.colors.primary}66`,
              padding: '16px',
            }}
          >
            <div style={{
              marginBottom: '12px',
              fontSize: '0.875rem',
              fontWeight: 600,
              color: APEX_TOKENS.colors.accent,
            }}>
              TERMINAL OUTPUT
            </div>

            <div style={{
              fontFamily: 'JetBrains Mono, monospace',
              fontSize: '13px',
              lineHeight: '1.4',
              color: APEX_TOKENS.colors.text,
            }}>
              <div style={{ color: APEX_TOKENS.colors.accent }}>
                apex@build:~/project$ npm run dev
              </div>
              <div style={{ color: APEX_TOKENS.colors.primary }}>
                ✓ Ready in 247ms
              </div>
              <div style={{ color: APEX_TOKENS.colors.text, opacity: 0.8 }}>
                ➜ Local: http://localhost:3000
              </div>
              <div style={{ color: APEX_TOKENS.colors.secondary }}>
                ➜ Network: http://192.168.1.100:3000
              </div>
              {isRunning && (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  style={{ color: APEX_TOKENS.colors.accent, marginTop: '8px' }}
                >
                  <APEXLoading variant="dots" size="sm" />
                  <span style={{ marginLeft: '12px' }}>Deploying to production...</span>
                </motion.div>
              )}
            </div>
          </APEXCard>
        </div>
      </div>
    </div>
  );
};

// ===== MAIN APP COMPONENT =====
const APEXBuildDemo: React.FC = () => {
  return (
    <APEXThemeProvider>
      <div className="apex-digital-rain">
        <APEXIDEDemo />
      </div>
    </APEXThemeProvider>
  );
};

export default APEXBuildDemo;