// APEX.BUILD Monaco Code Editor
// Advanced code editor with multi-AI integration

import React, { useEffect, useRef, useState, forwardRef } from 'react'
import * as monaco from 'monaco-editor'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { File, AICapability, AIProvider } from '@/types'
import { Button, Badge, Loading } from '@/components/ui'
import {
  Play,
  Save,
  Download,
  Zap,
  MessageSquare,
  Bug,
  RefreshCw,
  FileText,
  Sparkles
} from 'lucide-react'

// Editor theme configurations
const EDITOR_THEMES = {
  cyberpunk: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: '6272a4', fontStyle: 'italic' },
      { token: 'keyword', foreground: '00f5ff' },
      { token: 'string', foreground: '39ff14' },
      { token: 'number', foreground: 'ff6b9d' },
      { token: 'type', foreground: '00f5ff' },
      { token: 'function', foreground: 'ff6b9d' },
      { token: 'variable', foreground: 'ffffff' },
      { token: 'operator', foreground: '00f5ff' },
    ],
    colors: {
      'editor.background': '#0a0a0f',
      'editor.foreground': '#ffffff',
      'editor.lineHighlightBackground': '#001133',
      'editor.selectionBackground': '#00f5ff30',
      'editor.inactiveSelectionBackground': '#00f5ff15',
      'editorLineNumber.foreground': '#6272a4',
      'editorLineNumber.activeForeground': '#00f5ff',
      'editorCursor.foreground': '#00f5ff',
      'editorWhitespace.foreground': '#6272a420',
      'editorIndentGuide.background': '#6272a420',
      'editorIndentGuide.activeBackground': '#00f5ff40',
    },
  },
  matrix: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: '555555', fontStyle: 'italic' },
      { token: 'keyword', foreground: '39ff14' },
      { token: 'string', foreground: '00ff00' },
      { token: 'number', foreground: '39ff14' },
      { token: 'type', foreground: '39ff14' },
      { token: 'function', foreground: '00ff41' },
      { token: 'variable', foreground: 'ffffff' },
      { token: 'operator', foreground: '39ff14' },
    ],
    colors: {
      'editor.background': '#000000',
      'editor.foreground': '#00ff00',
      'editor.lineHighlightBackground': '#001100',
      'editor.selectionBackground': '#39ff1430',
      'editor.inactiveSelectionBackground': '#39ff1415',
      'editorLineNumber.foreground': '#555555',
      'editorLineNumber.activeForeground': '#39ff14',
      'editorCursor.foreground': '#39ff14',
    },
  },
  synthwave: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: '848bbd', fontStyle: 'italic' },
      { token: 'keyword', foreground: 'ff6b9d' },
      { token: 'string', foreground: 'c3e88d' },
      { token: 'number', foreground: 'f78c6c' },
      { token: 'type', foreground: 'ff6b9d' },
      { token: 'function', foreground: '82aaff' },
      { token: 'variable', foreground: 'ffffff' },
      { token: 'operator', foreground: 'ff6b9d' },
    ],
    colors: {
      'editor.background': '#2a2139',
      'editor.foreground': '#ffffff',
      'editor.lineHighlightBackground': '#3d2a4f',
      'editor.selectionBackground': '#ff6b9d30',
      'editor.inactiveSelectionBackground': '#ff6b9d15',
      'editorLineNumber.foreground': '#848bbd',
      'editorLineNumber.activeForeground': '#ff6b9d',
      'editorCursor.foreground': '#ff6b9d',
    },
  },
  neonCity: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: '5c7cfa', fontStyle: 'italic' },
      { token: 'keyword', foreground: '339af0' },
      { token: 'string', foreground: '51cf66' },
      { token: 'number', foreground: 'ffd43b' },
      { token: 'type', foreground: '339af0' },
      { token: 'function', foreground: '74c0fc' },
      { token: 'variable', foreground: 'ffffff' },
      { token: 'operator', foreground: '339af0' },
    ],
    colors: {
      'editor.background': '#0c1426',
      'editor.foreground': '#ffffff',
      'editor.lineHighlightBackground': '#1a2332',
      'editor.selectionBackground': '#339af030',
      'editor.inactiveSelectionBackground': '#339af015',
      'editorLineNumber.foreground': '#5c7cfa',
      'editorLineNumber.activeForeground': '#339af0',
      'editorCursor.foreground': '#339af0',
    },
  },
}

// Language configurations
const LANGUAGE_CONFIGS = {
  javascript: { id: 'javascript', defaultCode: 'console.log("Hello APEX.BUILD!");' },
  typescript: { id: 'typescript', defaultCode: 'console.log("Hello APEX.BUILD!");' },
  python: { id: 'python', defaultCode: 'print("Hello APEX.BUILD!")' },
  go: { id: 'go', defaultCode: 'package main\n\nimport "fmt"\n\nfunc main() {\n    fmt.Println("Hello APEX.BUILD!")\n}' },
  rust: { id: 'rust', defaultCode: 'fn main() {\n    println!("Hello APEX.BUILD!");\n}' },
  java: { id: 'java', defaultCode: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello APEX.BUILD!");\n    }\n}' },
  cpp: { id: 'cpp', defaultCode: '#include <iostream>\n\nint main() {\n    std::cout << "Hello APEX.BUILD!" << std::endl;\n    return 0;\n}' },
  html: { id: 'html', defaultCode: '<!DOCTYPE html>\n<html>\n<head>\n    <title>APEX.BUILD</title>\n</head>\n<body>\n    <h1>Hello APEX.BUILD!</h1>\n</body>\n</html>' },
  css: { id: 'css', defaultCode: 'body {\n    font-family: "Fira Code", monospace;\n    background: linear-gradient(135deg, #0a0a0f 0%, #001133 100%);\n    color: #00f5ff;\n}' },
  json: { id: 'json', defaultCode: '{\n  "name": "apex-build",\n  "version": "1.0.0",\n  "description": "Cyberpunk cloud development platform"\n}' },
}

export interface MonacoEditorProps {
  file?: File
  value?: string
  onChange?: (value: string) => void
  onSave?: (value: string) => void
  className?: string
  height?: string | number
  readOnly?: boolean
  showAIPanel?: boolean
  onAIRequest?: (capability: AICapability, prompt: string, code: string) => void
}

const MonacoEditor = forwardRef<monaco.editor.IStandaloneCodeEditor | null, MonacoEditorProps>(
  ({
    file,
    value = '',
    onChange,
    onSave,
    className,
    height = '100%',
    readOnly = false,
    showAIPanel = true,
    onAIRequest,
  }, ref) => {
    const editorRef = useRef<HTMLDivElement>(null)
    const [editor, setEditor] = useState<monaco.editor.IStandaloneCodeEditor | null>(null)
    const [isAILoading, setIsAILoading] = useState(false)
    const [aiCapability, setAICapability] = useState<AICapability>('code_completion')
    const [aiPrompt, setAIPrompt] = useState('')
    const [showAIPrompt, setShowAIPrompt] = useState(false)

    const { theme, currentProject } = useStore()

    // Initialize Monaco Editor
    useEffect(() => {
      if (!editorRef.current) return

      // Register custom themes
      Object.entries(EDITOR_THEMES).forEach(([themeName, themeData]) => {
        monaco.editor.defineTheme(`apex-${themeName}`, themeData)
      })

      // Create editor instance
      const editorInstance = monaco.editor.create(editorRef.current, {
        value: value || (file?.content || ''),
        language: getLanguageFromFile(file?.name || ''),
        theme: `apex-${theme.id}`,
        automaticLayout: true,
        readOnly,
        fontSize: 14,
        fontFamily: '"Fira Code", "SF Mono", Monaco, Menlo, Consolas, monospace',
        lineNumbers: 'on',
        minimap: { enabled: true },
        scrollBeyondLastLine: false,
        wordWrap: 'on',
        tabSize: 2,
        insertSpaces: true,
        folding: true,
        foldingHighlight: true,
        bracketPairColorization: { enabled: true },
        guides: {
          bracketPairs: true,
          indentation: true,
        },
        suggest: {
          showKeywords: true,
          showSnippets: true,
        },
        quickSuggestions: {
          other: true,
          comments: true,
          strings: true,
        },
      })

      // Set up event listeners
      editorInstance.onDidChangeModelContent(() => {
        const currentValue = editorInstance.getValue()
        onChange?.(currentValue)
      })

      // Keyboard shortcuts
      editorInstance.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
        onSave?.(editorInstance.getValue())
      })

      // AI integration shortcuts
      editorInstance.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyK, () => {
        setShowAIPrompt(true)
        setAICapability('code_completion')
      })

      editorInstance.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyK, () => {
        setShowAIPrompt(true)
        setAICapability('explanation')
      })

      setEditor(editorInstance)

      // Expose editor instance via ref
      if (typeof ref === 'function') {
        ref(editorInstance)
      } else if (ref) {
        ref.current = editorInstance
      }

      return () => {
        editorInstance.dispose()
      }
    }, [editorRef.current])

    // Update theme when it changes
    useEffect(() => {
      if (editor) {
        monaco.editor.setTheme(`apex-${theme.id}`)
      }
    }, [theme.id, editor])

    // Update content when file changes
    useEffect(() => {
      if (editor && file) {
        const model = editor.getModel()
        if (model) {
          model.setValue(file.content)
          const language = getLanguageFromFile(file.name)
          monaco.editor.setModelLanguage(model, language)
        }
      }
    }, [file, editor])

    // Get language from file extension
    const getLanguageFromFile = (filename: string): string => {
      const ext = filename.split('.').pop()?.toLowerCase() || ''
      const langMap: Record<string, string> = {
        'js': 'javascript',
        'jsx': 'javascript',
        'ts': 'typescript',
        'tsx': 'typescript',
        'py': 'python',
        'go': 'go',
        'rs': 'rust',
        'java': 'java',
        'cpp': 'cpp',
        'c': 'cpp',
        'html': 'html',
        'css': 'css',
        'scss': 'scss',
        'json': 'json',
        'md': 'markdown',
        'yaml': 'yaml',
        'yml': 'yaml',
        'xml': 'xml',
        'sql': 'sql',
      }
      return langMap[ext] || 'plaintext'
    }

    // Handle AI requests
    const handleAIRequest = async () => {
      if (!editor || !onAIRequest) return

      const selectedText = editor.getModel()?.getValueInRange(editor.getSelection()!)
      const fullCode = editor.getValue()
      const codeToAnalyze = selectedText || fullCode

      setIsAILoading(true)
      try {
        await onAIRequest(aiCapability, aiPrompt, codeToAnalyze)
      } finally {
        setIsAILoading(false)
        setShowAIPrompt(false)
        setAIPrompt('')
      }
    }

    // Execute code
    const handleExecute = () => {
      if (!editor || !currentProject) return

      const code = editor.getValue()
      const language = getLanguageFromFile(file?.name || '')

      // This would integrate with the execution service
      console.log('Executing code:', { code, language, projectId: currentProject.id })
    }

    return (
      <div className={cn('relative w-full h-full flex flex-col', className)}>
        {/* Editor Toolbar */}
        <div className="flex items-center justify-between p-3 bg-gray-900/80 backdrop-blur-md border-b border-gray-700/50">
          <div className="flex items-center space-x-2">
            {file && (
              <>
                <span className="text-sm font-medium text-white">{file.name}</span>
                <Badge variant="outline" size="xs">
                  {getLanguageFromFile(file.name)}
                </Badge>
                {file.is_locked && (
                  <Badge variant="warning" size="xs" icon="🔒">
                    Locked
                  </Badge>
                )}
              </>
            )}
          </div>

          <div className="flex items-center space-x-2">
            {showAIPanel && (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setShowAIPrompt(true)
                    setAICapability('code_completion')
                  }}
                  icon={<Sparkles size={14} />}
                >
                  AI Complete
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setShowAIPrompt(true)
                    setAICapability('debugging')
                  }}
                  icon={<Bug size={14} />}
                >
                  Debug
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setShowAIPrompt(true)
                    setAICapability('refactoring')
                  }}
                  icon={<RefreshCw size={14} />}
                >
                  Refactor
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setShowAIPrompt(true)
                    setAICapability('explanation')
                  }}
                  icon={<MessageSquare size={14} />}
                >
                  Explain
                </Button>
              </>
            )}

            <div className="w-px h-6 bg-gray-600" />

            <Button
              size="sm"
              variant="success"
              onClick={handleExecute}
              icon={<Play size={14} />}
            >
              Run
            </Button>

            <Button
              size="sm"
              variant="ghost"
              onClick={() => onSave?.(editor?.getValue() || '')}
              icon={<Save size={14} />}
            >
              Save
            </Button>
          </div>
        </div>

        {/* Monaco Editor Container */}
        <div className="flex-1 relative">
          <div
            ref={editorRef}
            className="w-full h-full"
            style={{ height: typeof height === 'string' ? height : `${height}px` }}
          />

          {/* AI Prompt Overlay */}
          {showAIPrompt && (
            <div className="absolute inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-10">
              <div className="bg-gray-900/95 backdrop-blur-md border border-cyan-400/50 rounded-lg p-6 max-w-md w-full mx-4">
                <h3 className="text-lg font-semibold text-white mb-4">
                  AI Assistant - {aiCapability.replace('_', ' ').toUpperCase()}
                </h3>

                <textarea
                  value={aiPrompt}
                  onChange={(e) => setAIPrompt(e.target.value)}
                  placeholder="Describe what you want the AI to help with..."
                  className="w-full h-24 bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-white placeholder:text-gray-400 focus:border-cyan-400 focus:outline-none resize-none"
                  autoFocus
                />

                <div className="flex justify-between items-center mt-4">
                  <select
                    value={aiCapability}
                    onChange={(e) => setAICapability(e.target.value as AICapability)}
                    className="bg-gray-800 border border-gray-600 rounded px-3 py-1 text-white text-sm focus:border-cyan-400 focus:outline-none"
                  >
                    <option value="code_completion">Complete Code</option>
                    <option value="code_generation">Generate Code</option>
                    <option value="debugging">Debug Code</option>
                    <option value="refactoring">Refactor Code</option>
                    <option value="explanation">Explain Code</option>
                    <option value="code_review">Review Code</option>
                    <option value="testing">Generate Tests</option>
                    <option value="documentation">Add Documentation</option>
                  </select>

                  <div className="flex space-x-2">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setShowAIPrompt(false)}
                    >
                      Cancel
                    </Button>
                    <Button
                      size="sm"
                      variant="primary"
                      onClick={handleAIRequest}
                      loading={isAILoading}
                      disabled={!aiPrompt.trim()}
                    >
                      Generate
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    )
  }
)

MonacoEditor.displayName = 'MonacoEditor'

export { MonacoEditor, EDITOR_THEMES, LANGUAGE_CONFIGS }