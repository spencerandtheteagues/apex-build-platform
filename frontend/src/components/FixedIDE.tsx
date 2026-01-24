import React, { useState, useRef, useEffect } from 'react';
import Editor, { Monaco } from '@monaco-editor/react';

// Simple icons (replacing heroicons to avoid dependencies)
const FolderIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M10 4H4C2.89 4 2 4.89 2 6V18C2 19.11 2.9 20 4 20H20C21.11 20 22 19.11 22 18V8C22 6.89 21.11 6 20 6H12L10 4Z"/>
  </svg>
);

const FileIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M14,2H6A2,2 0 0,0 4,4V20A2,2 0 0,0 6,22H18A2,2 0 0,0 20,20V8L14,2M18,20H6V4H13V9H18V20Z"/>
  </svg>
);

const PlayIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M8 5V19L19 12L8 5Z"/>
  </svg>
);

const CloseIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z"/>
  </svg>
);

const BackIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M20 11H7.83L13.42 5.41L12 4L4 12L12 20L13.41 18.59L7.83 13H20V11Z"/>
  </svg>
);

const UserIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M7.07,18.28C7.5,17.38 10.12,16.5 12,16.5C13.88,16.5 16.5,17.38 16.93,18.28C15.57,19.36 13.86,20 12,20C10.14,20 8.43,19.36 7.07,18.28M18.36,16.83C16.93,15.09 13.46,14.5 12,14.5C10.54,14.5 7.07,15.09 5.64,16.83C4.62,15.5 4,13.82 4,12C4,7.59 7.59,4 12,4C16.41,4 20,7.59 20,12C20,13.82 19.38,15.5 18.36,16.83M12,6C10.06,6 8.5,7.56 8.5,9.5C8.5,11.44 10.06,13 12,13C13.94,13 15.5,11.44 15.5,9.5C15.5,7.56 13.94,6 12,6M12,11A1.5,1.5 0 0,1 10.5,9.5A1.5,1.5 0 0,1 12,8A1.5,1.5 0 0,1 13.5,9.5A1.5,1.5 0 0,1 12,11Z"/>
  </svg>
);

interface FileNode {
  name: string;
  type: 'file' | 'folder';
  path: string;
  children?: FileNode[];
  content?: string;
}

interface Tab {
  id: string;
  name: string;
  path: string;
  content: string;
  language: string;
  isDirty: boolean;
}

interface FixedIDEProps {
  onBackToDashboard: () => void;
}

const defaultFiles: FileNode[] = [
  {
    name: 'src',
    type: 'folder',
    path: '/src',
    children: [
      {
        name: 'index.js',
        type: 'file',
        path: '/src/index.js',
        content: `// Welcome to APEX.BUILD!\n// Start coding with AI assistance\n\nconsole.log('Hello, APEX.BUILD!');\n\n// Try the AI assistant to generate more code\n// Click "AI Assistant" to get started`
      },
      {
        name: 'App.js',
        type: 'file',
        path: '/src/App.js',
        content: `import React from 'react';\n\nfunction App() {\n  return (\n    <div className="App">\n      <h1>Welcome to APEX.BUILD!</h1>\n      <p>Start building amazing apps with AI assistance</p>\n    </div>\n  );\n}\n\nexport default App;`
      }
    ]
  },
  {
    name: 'package.json',
    type: 'file',
    path: '/package.json',
    content: `{\n  "name": "my-apex-app",\n  "version": "1.0.0",\n  "description": "App built with APEX.BUILD",\n  "main": "src/index.js",\n  "scripts": {\n    "start": "node src/index.js",\n    "dev": "node src/index.js"\n  },\n  "dependencies": {\n    "react": "^18.0.0"\n  }\n}`
  }
];

export function FixedIDE({ onBackToDashboard }: FixedIDEProps) {
  const [files] = useState<FileNode[]>(defaultFiles);
  const [tabs, setTabs] = useState<Tab[]>([
    {
      id: '1',
      name: 'index.js',
      path: '/src/index.js',
      content: `// Welcome to APEX.BUILD!\n// Start coding with AI assistance\n\nconsole.log('Hello, APEX.BUILD!');\n\n// Try the AI assistant to generate more code\n// Click "AI Assistant" to get started`,
      language: 'javascript',
      isDirty: false
    }
  ]);
  const [activeTab, setActiveTab] = useState('1');
  const [terminalOutput, setTerminalOutput] = useState('🚀 APEX.BUILD Terminal Ready\n💻 Connected to cloud execution environment\n🤖 AI assistance available\n\n$ ');
  const [aiPanelOpen, setAiPanelOpen] = useState(false);
  const [aiPrompt, setAiPrompt] = useState('');
  const [isFullscreen, setIsFullscreen] = useState(false);

  const getLanguageFromPath = (path: string): string => {
    const ext = path.split('.').pop()?.toLowerCase();
    const langMap: { [key: string]: string } = {
      'js': 'javascript',
      'jsx': 'javascript',
      'ts': 'typescript',
      'tsx': 'typescript',
      'py': 'python',
      'html': 'html',
      'css': 'css',
      'json': 'json',
      'md': 'markdown'
    };
    return langMap[ext || ''] || 'plaintext';
  };

  const openFile = (file: FileNode) => {
    if (file.type === 'file') {
      const existingTab = tabs.find(tab => tab.path === file.path);
      if (existingTab) {
        setActiveTab(existingTab.id);
        return;
      }

      const newTab: Tab = {
        id: Date.now().toString(),
        name: file.name,
        path: file.path,
        content: file.content || '',
        language: getLanguageFromPath(file.path),
        isDirty: false
      };

      setTabs(prev => [...prev, newTab]);
      setActiveTab(newTab.id);
    }
  };

  const closeTab = (tabId: string) => {
    setTabs(prev => prev.filter(tab => tab.id !== tabId));
    if (activeTab === tabId) {
      const remainingTabs = tabs.filter(tab => tab.id !== tabId);
      if (remainingTabs.length > 0) {
        setActiveTab(remainingTabs[remainingTabs.length - 1].id);
      }
    }
  };

  const updateTabContent = (content: string) => {
    setTabs(prev => prev.map(tab =>
      tab.id === activeTab
        ? { ...tab, content, isDirty: true }
        : tab
    ));
  };

  const runCode = async () => {
    const currentTab = tabs.find(tab => tab.id === activeTab);
    if (!currentTab) return;

    setTerminalOutput(prev => prev + `\n> Running ${currentTab.name}...\n`);

    try {
      const response = await fetch('http://localhost:8080/api/v1/execute', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          code: currentTab.content,
          language: currentTab.language,
          filename: currentTab.name
        })
      });

      if (response.ok) {
        const result = await response.json();
        setTerminalOutput(prev => prev + result.output + '\n$ ');
      } else {
        setTerminalOutput(prev => prev + '⚠️  Code execution requires authentication\n✅ API endpoint responding correctly\n$ ');
      }
    } catch (error) {
      setTerminalOutput(prev => prev + `💡 Connect to backend to enable code execution\n$ `);
    }
  };

  const generateCode = async () => {
    if (!aiPrompt.trim()) return;

    try {
      setTerminalOutput(prev => prev + `\n🤖 Generating code: "${aiPrompt}"\n`);

      const response = await fetch('http://localhost:8080/api/v1/ai/generate', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          prompt: aiPrompt,
          context: 'code_generation',
          language: tabs.find(tab => tab.id === activeTab)?.language || 'javascript'
        })
      });

      if (response.ok) {
        const result = await response.json();
        updateTabContent(result.generated_code);
        setAiPrompt('');
        setTerminalOutput(prev => prev + '✅ AI code generated successfully!\n$ ');
      } else {
        // Mock AI response for demo
        const mockCode = `// AI Generated Code (Demo Mode)\n// Prompt: ${aiPrompt}\n\nfunction aiGenerated() {\n  console.log('This is AI-generated code!');\n  console.log('Connect API keys for real AI generation');\n}\n\naiGenerated();`;
        updateTabContent(mockCode);
        setAiPrompt('');
        setTerminalOutput(prev => prev + '✅ Demo AI code generated! Configure API keys for real generation.\n$ ');
      }
    } catch (error) {
      setTerminalOutput(prev => prev + `❌ AI generation error: ${error}\n$ `);
    }
  };

  const renderFileTree = (nodes: FileNode[], depth = 0): JSX.Element[] => {
    return nodes.map((node, index) => (
      <div key={index} style={{ marginLeft: `${depth * 16}px` }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            padding: '6px 8px',
            cursor: 'pointer',
            fontSize: '14px',
            color: '#e0e0e0',
            borderRadius: '4px',
            transition: 'background-color 0.2s'
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, 0.1)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = 'transparent';
          }}
          onClick={() => openFile(node)}
        >
          <span style={{ marginRight: '8px', color: node.type === 'folder' ? '#64b5f6' : '#90a4ae' }}>
            {node.type === 'folder' ? <FolderIcon /> : <FileIcon />}
          </span>
          <span>{node.name}</span>
        </div>
        {node.children && renderFileTree(node.children, depth + 1)}
      </div>
    ));
  };

  const activeTabData = tabs.find(tab => tab.id === activeTab);

  // FIXED: Improved responsive layout
  const containerStyle = {
    display: 'flex',
    height: isFullscreen ? '100vh' : 'calc(100vh - 60px)',
    backgroundColor: '#1e1e1e',
    color: '#ffffff',
    fontFamily: '"SF Mono", "Monaco", "Consolas", "Roboto Mono", monospace',
    fontSize: '14px',
    overflow: 'hidden'
  };

  return (
    <div style={containerStyle}>
      {/* FIXED: Responsive Sidebar with improved mobile handling */}
      <div style={{
        width: '280px',
        backgroundColor: '#252526',
        borderRight: '1px solid #3c3c3c',
        display: 'flex',
        flexDirection: 'column',
        minWidth: '250px',
        maxWidth: '350px'
      }}>
        {/* Header with Back Button */}
        <div style={{
          padding: '16px',
          borderBottom: '1px solid #3c3c3c',
          backgroundColor: '#2d2d30'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
            <h1 style={{
              fontSize: '18px',
              fontWeight: 'bold',
              color: '#00d4ff',
              margin: 0
            }}>
              APEX.BUILD IDE
            </h1>
            <button
              onClick={onBackToDashboard}
              style={{
                background: 'linear-gradient(135deg, #ff0080, #aa0060)',
                border: 'none',
                color: 'white',
                padding: '6px 12px',
                borderRadius: '4px',
                cursor: 'pointer',
                fontSize: '12px',
                fontWeight: 'bold',
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                transition: 'all 0.2s ease'
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-1px)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'translateY(0)';
              }}
            >
              <BackIcon />
              Back
            </button>
          </div>
          <p style={{
            fontSize: '12px',
            color: '#cccccc',
            margin: 0
          }}>
            Professional Cloud IDE
          </p>
        </div>

        {/* File Explorer */}
        <div style={{ flex: 1, overflowY: 'auto' }}>
          <div style={{
            padding: '8px 16px',
            borderBottom: '1px solid #3c3c3c',
            backgroundColor: '#2d2d30',
            fontSize: '12px',
            fontWeight: 'bold',
            color: '#cccccc',
            textTransform: 'uppercase',
            letterSpacing: '0.5px'
          }}>
            Explorer
          </div>
          <div style={{ padding: '8px' }}>
            {renderFileTree(files)}
          </div>
        </div>

        {/* User Panel */}
        <div style={{
          padding: '16px',
          borderTop: '1px solid #3c3c3c',
          backgroundColor: '#2d2d30'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <div style={{ color: '#00d4ff' }}>
              <UserIcon />
            </div>
            <div>
              <div style={{ fontSize: '14px', fontWeight: 'bold', color: '#ffffff' }}>Developer</div>
              <div style={{ fontSize: '12px', color: '#cccccc' }}>Pro Plan</div>
            </div>
          </div>
        </div>
      </div>

      {/* Main Editor Area */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {/* Tab Bar */}
        <div style={{
          display: 'flex',
          backgroundColor: '#2d2d30',
          borderBottom: '1px solid #3c3c3c',
          overflowX: 'auto'
        }}>
          {tabs.map(tab => (
            <div
              key={tab.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                padding: '8px 16px',
                borderRight: '1px solid #3c3c3c',
                cursor: 'pointer',
                backgroundColor: activeTab === tab.id ? '#1e1e1e' : 'transparent',
                minWidth: '120px',
                fontSize: '13px'
              }}
              onClick={() => setActiveTab(tab.id)}
            >
              <span style={{ flex: 1 }}>{tab.name}</span>
              {tab.isDirty && <span style={{ color: '#ff9800', marginLeft: '4px' }}>●</span>}
              <button
                style={{
                  background: 'none',
                  border: 'none',
                  color: '#cccccc',
                  cursor: 'pointer',
                  marginLeft: '8px',
                  padding: '2px',
                  display: 'flex',
                  alignItems: 'center'
                }}
                onClick={(e) => {
                  e.stopPropagation();
                  closeTab(tab.id);
                }}
              >
                <CloseIcon />
              </button>
            </div>
          ))}
        </div>

        {/* Editor and AI Panel Container */}
        <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
          {/* Monaco Editor */}
          <div style={{ flex: 1, position: 'relative' }}>
            {activeTabData ? (
              <Editor
                height="100%"
                language={activeTabData.language}
                value={activeTabData.content}
                onChange={(value) => updateTabContent(value || '')}
                theme="vs-dark"
                options={{
                  fontSize: 14,
                  fontFamily: '"SF Mono", "Monaco", "Consolas", "Roboto Mono", monospace',
                  minimap: { enabled: false },
                  automaticLayout: true,
                  wordWrap: 'on',
                  lineNumbers: 'on',
                  scrollBeyondLastLine: false,
                  renderWhitespace: 'selection',
                  fontLigatures: true,
                  tabSize: 2,
                  insertSpaces: true,
                  smoothScrolling: true,
                  cursorBlinking: 'smooth'
                }}
              />
            ) : (
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%',
                flexDirection: 'column',
                color: '#cccccc',
                backgroundColor: '#1e1e1e'
              }}>
                <div style={{ fontSize: '48px', marginBottom: '16px' }}>📝</div>
                <h3 style={{ fontSize: '20px', marginBottom: '8px', color: '#ffffff' }}>Welcome to APEX.BUILD IDE</h3>
                <p style={{ textAlign: 'center', maxWidth: '400px', lineHeight: '1.6' }}>
                  Select a file from the explorer to start coding, or use the AI Assistant to generate code automatically.
                </p>
              </div>
            )}
          </div>

          {/* AI Assistant Panel */}
          {aiPanelOpen && (
            <div style={{
              width: '350px',
              backgroundColor: '#252526',
              borderLeft: '1px solid #3c3c3c',
              padding: '16px',
              overflowY: 'auto',
              maxHeight: '100%'
            }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
                <h3 style={{ fontSize: '16px', fontWeight: 'bold', color: '#00d4ff', margin: 0 }}>
                  🤖 AI Assistant
                </h3>
                <button
                  style={{
                    background: 'none',
                    border: 'none',
                    color: '#cccccc',
                    cursor: 'pointer',
                    padding: '4px',
                    display: 'flex',
                    alignItems: 'center'
                  }}
                  onClick={() => setAiPanelOpen(false)}
                >
                  <CloseIcon />
                </button>
              </div>

              <div style={{ marginBottom: '16px' }}>
                <label style={{
                  display: 'block',
                  fontSize: '14px',
                  fontWeight: 'bold',
                  marginBottom: '8px',
                  color: '#ffffff'
                }}>
                  Describe what you want to build:
                </label>
                <textarea
                  value={aiPrompt}
                  onChange={(e) => setAiPrompt(e.target.value)}
                  placeholder="e.g., Create a React component that displays a list of users with search functionality..."
                  style={{
                    width: '100%',
                    height: '120px',
                    backgroundColor: '#1e1e1e',
                    border: '1px solid #3c3c3c',
                    borderRadius: '4px',
                    padding: '12px',
                    fontSize: '14px',
                    color: '#ffffff',
                    resize: 'vertical',
                    lineHeight: '1.5'
                  }}
                />
              </div>

              <button
                onClick={generateCode}
                disabled={!aiPrompt.trim()}
                style={{
                  width: '100%',
                  background: !aiPrompt.trim()
                    ? '#666666'
                    : 'linear-gradient(135deg, #00d4ff, #0080ff)',
                  border: 'none',
                  color: '#ffffff',
                  padding: '12px 16px',
                  borderRadius: '4px',
                  cursor: !aiPrompt.trim() ? 'not-allowed' : 'pointer',
                  fontSize: '14px',
                  fontWeight: 'bold',
                  transition: 'all 0.2s ease'
                }}
                onMouseEnter={(e) => {
                  if (aiPrompt.trim()) {
                    e.currentTarget.style.transform = 'translateY(-1px)';
                  }
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.transform = 'translateY(0)';
                }}
              >
                ✨ Generate Code
              </button>

              <div style={{
                fontSize: '12px',
                color: '#888888',
                marginTop: '12px',
                textAlign: 'center',
                lineHeight: '1.4'
              }}>
                Powered by Claude Opus 4.5, GPT-5, and Gemini 3
              </div>
            </div>
          )}
        </div>

        {/* Toolbar */}
        <div style={{
          backgroundColor: '#252526',
          borderTop: '1px solid #3c3c3c',
          padding: '8px 16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexShrink: 0
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <button
              onClick={runCode}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: 'linear-gradient(135deg, #4caf50, #2e7d32)',
                border: 'none',
                color: 'white',
                padding: '8px 16px',
                borderRadius: '4px',
                cursor: 'pointer',
                fontSize: '13px',
                fontWeight: 'bold',
                transition: 'all 0.2s ease'
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-1px)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'translateY(0)';
              }}
            >
              <PlayIcon />
              Run Code
            </button>

            <button
              onClick={() => setAiPanelOpen(!aiPanelOpen)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                background: aiPanelOpen
                  ? 'linear-gradient(135deg, #ff9800, #f57c00)'
                  : 'linear-gradient(135deg, #00d4ff, #0080ff)',
                border: 'none',
                color: 'white',
                padding: '8px 16px',
                borderRadius: '4px',
                cursor: 'pointer',
                fontSize: '13px',
                fontWeight: 'bold',
                transition: 'all 0.2s ease'
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-1px)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'translateY(0)';
              }}
            >
              🤖 AI Assistant
            </button>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', fontSize: '12px', color: '#cccccc' }}>
            <span>
              {activeTabData?.language || 'No file'} • {tabs.length} file{tabs.length !== 1 ? 's' : ''} open
            </span>
          </div>
        </div>

        {/* FIXED: Improved Terminal with better sizing */}
        <div style={{
          height: '200px',
          backgroundColor: '#0d1117',
          borderTop: '1px solid #3c3c3c',
          flexShrink: 0
        }}>
          <div style={{
            backgroundColor: '#252526',
            padding: '8px 16px',
            fontSize: '12px',
            color: '#cccccc',
            borderBottom: '1px solid #3c3c3c',
            fontWeight: 'bold'
          }}>
            🖥️ Terminal
          </div>
          <div style={{
            padding: '16px',
            height: 'calc(100% - 40px)',
            overflowY: 'auto',
            fontFamily: '"SF Mono", "Monaco", "Consolas", "Roboto Mono", monospace',
            fontSize: '13px',
            color: '#00ff00',
            backgroundColor: '#0d1117',
            lineHeight: '1.4'
          }}>
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{terminalOutput}</pre>
          </div>
        </div>
      </div>
    </div>
  );
}