// APEX.BUILD AI Chat Panel
// Context-aware chat with @mentions, multi-turn conversations, and code insertion

import React, { useState, useRef, useEffect, useCallback } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import { aiService, StreamCallback, AIChatContext } from '@/services/aiService'
import {
  AIMessage,
  AIProvider,
  AICapability,
  File
} from '@/types'
import {
  Button,
  Card,
  Badge,
  Avatar,
  Loading,
} from '@/components/ui'
import {
  Send,
  Sparkles,
  Code,
  Bug,
  RefreshCw,
  FileText,
  MessageSquare,
  Copy,
  Check,
  X,
  Plus,
  Trash2,
  ChevronDown,
  AtSign,
  FileCode,
  Braces,
  Wand2,
  TestTube,
  BookOpen,
  Shield,
  Zap,
  Settings,
} from 'lucide-react'
import { formatRelativeTime, generateId } from '@/lib/utils'

export interface AIChatPanelProps {
  className?: string
  projectId?: number
  fileId?: number
  fileName?: string
  language?: string
  currentCode?: string
  selectedCode?: string
  cursorPosition?: { line: number; column: number }
  onCodeInsert?: (code: string) => void
  onCodeReplace?: (code: string) => void
  isCompact?: boolean
}

interface Conversation {
  id: string
  title: string
  messages: AIMessage[]
  context: AIChatContext
  createdAt: string
  updatedAt: string
}

interface MentionSuggestion {
  type: 'file' | 'function' | 'symbol'
  name: string
  path?: string
  icon: React.ReactNode
}

const QUICK_ACTIONS = [
  { id: 'explain', label: 'Explain', icon: <MessageSquare size={14} />, capability: 'explanation' as AICapability },
  { id: 'debug', label: 'Debug', icon: <Bug size={14} />, capability: 'debugging' as AICapability },
  { id: 'refactor', label: 'Refactor', icon: <RefreshCw size={14} />, capability: 'refactoring' as AICapability },
  { id: 'test', label: 'Tests', icon: <TestTube size={14} />, capability: 'testing' as AICapability },
  { id: 'docs', label: 'Docs', icon: <BookOpen size={14} />, capability: 'documentation' as AICapability },
  { id: 'review', label: 'Review', icon: <Shield size={14} />, capability: 'code_review' as AICapability },
]

const PROVIDER_OPTIONS = [
  { id: 'auto', label: 'Auto', icon: <Zap size={14} /> },
  { id: 'claude', label: 'Claude', icon: <Sparkles size={14} /> },
  { id: 'gpt4', label: 'GPT-4', icon: <Braces size={14} /> },
  { id: 'gemini', label: 'Gemini', icon: <Wand2 size={14} /> },
]

export const AIChatPanel: React.FC<AIChatPanelProps> = ({
  className,
  projectId,
  fileId,
  fileName,
  language,
  currentCode,
  selectedCode,
  cursorPosition,
  onCodeInsert,
  onCodeReplace,
  isCompact = false,
}) => {
  // State
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null)
  const [inputMessage, setInputMessage] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [streamingContent, setStreamingContent] = useState('')
  const [selectedProvider, setSelectedProvider] = useState<AIProvider | 'auto'>('auto')
  const [showMentions, setShowMentions] = useState(false)
  const [mentionQuery, setMentionQuery] = useState('')
  const [mentionPosition, setMentionPosition] = useState(0)
  const [copiedMessageId, setCopiedMessageId] = useState<string | null>(null)
  const [showProviderDropdown, setShowProviderDropdown] = useState(false)

  // Refs
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const chatContainerRef = useRef<HTMLDivElement>(null)

  // Store
  const { user, files, currentProject } = useStore()

  // Get active conversation
  const activeConversation = conversations.find(c => c.id === activeConversationId)

  // Scroll to bottom
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [activeConversation?.messages, streamingContent, scrollToBottom])

  // Create new conversation
  const createConversation = useCallback(() => {
    const newConversation: Conversation = {
      id: generateId(),
      title: 'New Chat',
      messages: [],
      context: {
        projectId,
        fileId,
        fileName,
        language,
        currentCode,
        selectedCode,
        cursorPosition,
      },
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }

    setConversations(prev => [newConversation, ...prev])
    setActiveConversationId(newConversation.id)
  }, [projectId, fileId, fileName, language, currentCode, selectedCode, cursorPosition])

  // Update context when props change
  useEffect(() => {
    if (activeConversationId) {
      setConversations(prev => prev.map(conv =>
        conv.id === activeConversationId
          ? {
              ...conv,
              context: {
                ...conv.context,
                projectId,
                fileId,
                fileName,
                language,
                currentCode,
                selectedCode,
                cursorPosition,
              },
            }
          : conv
      ))
    }
  }, [activeConversationId, projectId, fileId, fileName, language, currentCode, selectedCode, cursorPosition])

  // Handle sending message
  const sendMessage = useCallback(async (overridePrompt?: string, capability?: AICapability) => {
    const message = overridePrompt || inputMessage.trim()
    if (!message || isLoading) return

    // Create conversation if needed
    let conversationId = activeConversationId
    if (!conversationId) {
      createConversation()
      conversationId = conversations[0]?.id || generateId()
    }

    // Add user message
    const userMessage: AIMessage = {
      id: generateId(),
      type: 'user',
      content: message,
      timestamp: new Date().toISOString(),
    }

    setConversations(prev => prev.map(conv =>
      conv.id === conversationId
        ? {
            ...conv,
            messages: [...conv.messages, userMessage],
            title: conv.messages.length === 0 ? message.substring(0, 50) : conv.title,
            updatedAt: new Date().toISOString(),
          }
        : conv
    ))

    setInputMessage('')
    setIsLoading(true)
    setStreamingContent('')

    try {
      // Build context with mentions
      const mentions = aiService.parseMentions(message)
      const referencedFiles: Array<{ path: string; content: string }> = []

      for (const mention of mentions) {
        if (mention.type === 'file') {
          const file = files.find(f => f.path.includes(mention.name) || f.name === mention.name)
          if (file) {
            referencedFiles.push({ path: file.path, content: file.content })
          }
        }
      }

      const context: AIChatContext = {
        projectId,
        fileId,
        fileName,
        language,
        currentCode,
        selectedCode,
        cursorPosition,
        referencedFiles,
        conversationHistory: activeConversation?.messages,
      }

      // Stream response
      const streamCallback: StreamCallback = (chunk) => {
        if (chunk.type === 'content' && chunk.content) {
          setStreamingContent(prev => prev + chunk.content)
        } else if (chunk.type === 'done') {
          // Finalize message
          setStreamingContent('')
        }
      }

      const content = await aiService.generate({
        capability: capability || 'code_generation',
        prompt: message,
        code: selectedCode || currentCode,
        language,
        context,
        provider: selectedProvider,
        stream: true,
      }, streamCallback)

      // Add AI response
      const aiMessage: AIMessage = {
        id: generateId(),
        type: 'assistant',
        content,
        provider: selectedProvider === 'auto' ? 'claude' : selectedProvider,
        capability: capability || 'code_generation',
        code: extractCodeFromContent(content),
        language,
        timestamp: new Date().toISOString(),
      }

      setConversations(prev => prev.map(conv =>
        conv.id === conversationId
          ? {
              ...conv,
              messages: [...conv.messages, aiMessage],
              updatedAt: new Date().toISOString(),
            }
          : conv
      ))
    } catch (error) {
      // Add error message
      const errorMessage: AIMessage = {
        id: generateId(),
        type: 'system',
        content: `Error: ${error instanceof Error ? error.message : 'Failed to generate response'}`,
        timestamp: new Date().toISOString(),
        error: error instanceof Error ? error.message : 'Unknown error',
      }

      setConversations(prev => prev.map(conv =>
        conv.id === conversationId
          ? { ...conv, messages: [...conv.messages, errorMessage] }
          : conv
      ))
    } finally {
      setIsLoading(false)
      setStreamingContent('')
    }
  }, [
    inputMessage,
    isLoading,
    activeConversationId,
    activeConversation,
    conversations,
    createConversation,
    projectId,
    fileId,
    fileName,
    language,
    currentCode,
    selectedCode,
    cursorPosition,
    selectedProvider,
    files,
  ])

  // Extract code blocks from content
  const extractCodeFromContent = (content: string): string | undefined => {
    const codeBlockRegex = /```[\w]*\n([\s\S]*?)\n```/g
    const matches = content.match(codeBlockRegex)
    if (matches && matches.length > 0) {
      return matches[0].replace(/```[\w]*\n|```$/g, '').trim()
    }
    return undefined
  }

  // Handle quick action
  const handleQuickAction = useCallback((action: typeof QUICK_ACTIONS[0]) => {
    const codeToUse = selectedCode || currentCode
    if (!codeToUse) {
      sendMessage(`${action.label} the current code`, action.capability)
      return
    }

    const prompts: Record<string, string> = {
      explain: 'Explain this code in detail:',
      debug: 'Debug this code and identify issues:',
      refactor: 'Refactor this code for better readability and performance:',
      test: 'Generate comprehensive unit tests for this code:',
      docs: 'Generate documentation for this code:',
      review: 'Review this code for bugs, security issues, and best practices:',
    }

    sendMessage(prompts[action.id], action.capability)
  }, [selectedCode, currentCode, sendMessage])

  // Handle copy code
  const handleCopyCode = useCallback(async (code: string, messageId: string) => {
    try {
      await navigator.clipboard.writeText(code)
      setCopiedMessageId(messageId)
      setTimeout(() => setCopiedMessageId(null), 2000)
    } catch (error) {
      console.error('Failed to copy:', error)
    }
  }, [])

  // Handle insert code
  const handleInsertCode = useCallback((code: string) => {
    if (onCodeInsert) {
      onCodeInsert(code)
    }
  }, [onCodeInsert])

  // Handle replace code
  const handleReplaceCode = useCallback((code: string) => {
    if (onCodeReplace) {
      onCodeReplace(code)
    }
  }, [onCodeReplace])

  // Handle keyboard shortcuts
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      if (e.shiftKey) {
        // Allow new line
        return
      } else {
        e.preventDefault()
        sendMessage()
      }
    }

    // Handle @ mentions
    if (e.key === '@') {
      setShowMentions(true)
      setMentionPosition(inputRef.current?.selectionStart || 0)
    }

    if (showMentions && e.key === 'Escape') {
      setShowMentions(false)
    }
  }, [sendMessage, showMentions])

  // Handle input change
  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value
    setInputMessage(value)

    // Check for @ mention
    const cursorPos = e.target.selectionStart
    const textBeforeCursor = value.substring(0, cursorPos)
    const lastAtIndex = textBeforeCursor.lastIndexOf('@')

    if (lastAtIndex !== -1 && lastAtIndex === textBeforeCursor.length - 1) {
      setShowMentions(true)
      setMentionPosition(lastAtIndex)
      setMentionQuery('')
    } else if (lastAtIndex !== -1 && !textBeforeCursor.substring(lastAtIndex).includes(' ')) {
      setMentionQuery(textBeforeCursor.substring(lastAtIndex + 1))
    } else {
      setShowMentions(false)
    }
  }, [])

  // Get mention suggestions
  const getMentionSuggestions = useCallback((): MentionSuggestion[] => {
    const suggestions: MentionSuggestion[] = []

    // Add file suggestions
    files.forEach(file => {
      if (file.type === 'file' && (
        !mentionQuery ||
        file.name.toLowerCase().includes(mentionQuery.toLowerCase()) ||
        file.path.toLowerCase().includes(mentionQuery.toLowerCase())
      )) {
        suggestions.push({
          type: 'file',
          name: file.name,
          path: file.path,
          icon: <FileCode size={14} />,
        })
      }
    })

    return suggestions.slice(0, 10)
  }, [files, mentionQuery])

  // Handle mention selection
  const handleMentionSelect = useCallback((suggestion: MentionSuggestion) => {
    const beforeMention = inputMessage.substring(0, mentionPosition)
    const afterMention = inputMessage.substring(mentionPosition + 1 + mentionQuery.length)
    const mentionText = `@${suggestion.type}:${suggestion.name} `

    setInputMessage(beforeMention + mentionText + afterMention)
    setShowMentions(false)
    inputRef.current?.focus()
  }, [inputMessage, mentionPosition, mentionQuery])

  // Delete conversation
  const deleteConversation = useCallback((id: string) => {
    setConversations(prev => prev.filter(c => c.id !== id))
    if (activeConversationId === id) {
      const remaining = conversations.filter(c => c.id !== id)
      setActiveConversationId(remaining[0]?.id || null)
    }
  }, [activeConversationId, conversations])

  // Render message content with code highlighting
  const renderMessageContent = (message: AIMessage) => {
    const content = message.content
    const parts: React.ReactNode[] = []
    let lastIndex = 0

    // Find code blocks
    const codeBlockRegex = /```(\w*)\n([\s\S]*?)```/g
    let match

    while ((match = codeBlockRegex.exec(content)) !== null) {
      // Add text before code block
      if (match.index > lastIndex) {
        parts.push(
          <span key={`text-${lastIndex}`} className="whitespace-pre-wrap">
            {content.substring(lastIndex, match.index)}
          </span>
        )
      }

      const codeLanguage = match[1] || language || 'plaintext'
      const codeContent = match[2]

      // Add code block
      parts.push(
        <div key={`code-${match.index}`} className="my-3 rounded-lg overflow-hidden border border-gray-700/50">
          <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700/50">
            <div className="flex items-center gap-2">
              <Code size={14} className="text-gray-400" />
              <span className="text-xs text-gray-400">{codeLanguage}</span>
            </div>
            <div className="flex items-center gap-1">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => handleCopyCode(codeContent, message.id)}
                icon={copiedMessageId === message.id ? <Check size={12} /> : <Copy size={12} />}
              >
                {copiedMessageId === message.id ? 'Copied' : 'Copy'}
              </Button>
              {onCodeInsert && (
                <Button
                  size="xs"
                  variant="ghost"
                  onClick={() => handleInsertCode(codeContent)}
                  icon={<Plus size={12} />}
                >
                  Insert
                </Button>
              )}
              {onCodeReplace && selectedCode && (
                <Button
                  size="xs"
                  variant="ghost"
                  onClick={() => handleReplaceCode(codeContent)}
                  icon={<RefreshCw size={12} />}
                >
                  Replace
                </Button>
              )}
            </div>
          </div>
          <pre className="p-3 bg-gray-900/50 overflow-x-auto">
            <code className="text-sm text-gray-100 font-mono">
              {codeContent}
            </code>
          </pre>
        </div>
      )

      lastIndex = match.index + match[0].length
    }

    // Add remaining text
    if (lastIndex < content.length) {
      parts.push(
        <span key={`text-${lastIndex}`} className="whitespace-pre-wrap">
          {content.substring(lastIndex)}
        </span>
      )
    }

    return parts.length > 0 ? parts : <span className="whitespace-pre-wrap">{content}</span>
  }

  return (
    <div className={cn('flex flex-col h-full bg-gray-900/50', className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-gray-800/50 border-b border-gray-700/50">
        <div className="flex items-center gap-2">
          <Sparkles className="w-5 h-5 text-cyan-400" />
          <span className="font-semibold text-white">AI Assistant</span>
          {fileName && (
            <Badge variant="outline" size="xs">
              {fileName}
            </Badge>
          )}
        </div>

        <div className="flex items-center gap-2">
          {/* Provider selector */}
          <div className="relative">
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setShowProviderDropdown(!showProviderDropdown)}
              icon={PROVIDER_OPTIONS.find(p => p.id === selectedProvider)?.icon}
            >
              {PROVIDER_OPTIONS.find(p => p.id === selectedProvider)?.label}
              <ChevronDown size={12} className="ml-1" />
            </Button>

            {showProviderDropdown && (
              <div className="absolute right-0 mt-1 w-40 bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-50">
                {PROVIDER_OPTIONS.map(provider => (
                  <button
                    key={provider.id}
                    onClick={() => {
                      setSelectedProvider(provider.id as AIProvider | 'auto')
                      setShowProviderDropdown(false)
                    }}
                    className={cn(
                      'w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-700 transition-colors',
                      selectedProvider === provider.id && 'bg-gray-700 text-cyan-400'
                    )}
                  >
                    {provider.icon}
                    {provider.label}
                  </button>
                ))}
              </div>
            )}
          </div>

          <Button
            size="sm"
            variant="ghost"
            onClick={createConversation}
            icon={<Plus size={14} />}
            title="New Chat"
          />
        </div>
      </div>

      {/* Conversation tabs */}
      {conversations.length > 0 && !isCompact && (
        <div className="flex items-center gap-1 px-2 py-1 bg-gray-800/30 border-b border-gray-700/50 overflow-x-auto">
          {conversations.slice(0, 5).map(conv => (
            <div
              key={conv.id}
              className={cn(
                'flex items-center gap-1 px-2 py-1 rounded cursor-pointer group min-w-0 max-w-32',
                activeConversationId === conv.id
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:bg-gray-700/50 hover:text-gray-200'
              )}
              onClick={() => setActiveConversationId(conv.id)}
            >
              <span className="text-xs truncate">{conv.title}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  deleteConversation(conv.id)
                }}
                className="opacity-0 group-hover:opacity-100 hover:text-red-400 transition-opacity"
              >
                <X size={12} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Quick actions */}
      {(selectedCode || currentCode) && (
        <div className="flex items-center gap-1 px-3 py-2 bg-gray-800/20 border-b border-gray-700/50 overflow-x-auto">
          {QUICK_ACTIONS.map(action => (
            <Button
              key={action.id}
              size="xs"
              variant="ghost"
              onClick={() => handleQuickAction(action)}
              icon={action.icon}
              className="flex-shrink-0"
            >
              {action.label}
            </Button>
          ))}
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4" ref={chatContainerRef}>
        {activeConversation ? (
          <>
            {activeConversation.messages.map(message => (
              <div
                key={message.id}
                className={cn(
                  'flex gap-3',
                  message.type === 'user' && 'flex-row-reverse'
                )}
              >
                <Avatar
                  size="sm"
                  variant={message.type === 'user' ? 'cyberpunk' : 'matrix'}
                  fallback={message.type === 'user' ? user?.username : 'AI'}
                  src={message.type === 'user' ? user?.avatar_url : undefined}
                />

                <div className={cn(
                  'flex flex-col gap-1 max-w-[80%]',
                  message.type === 'user' && 'items-end'
                )}>
                  <div className="flex items-center gap-2 text-xs text-gray-400">
                    <span>{message.type === 'user' ? user?.username : 'AI Assistant'}</span>
                    {message.provider && (
                      <Badge variant="outline" size="xs">{message.provider}</Badge>
                    )}
                    <span>{formatRelativeTime(message.timestamp)}</span>
                  </div>

                  <Card
                    variant={
                      message.type === 'system' ? 'neutral' :
                      message.type === 'user' ? 'cyberpunk' :
                      message.error ? 'error' : 'matrix'
                    }
                    padding="sm"
                    className={cn(
                      message.type === 'user' && 'bg-cyan-900/30',
                      message.type === 'assistant' && 'bg-gray-800/50',
                      message.error && 'bg-red-900/20 border-red-500/30'
                    )}
                  >
                    <div className="text-sm text-gray-100">
                      {renderMessageContent(message)}
                    </div>
                  </Card>
                </div>
              </div>
            ))}

            {/* Streaming content */}
            {isLoading && streamingContent && (
              <div className="flex gap-3">
                <Avatar size="sm" variant="matrix" fallback="AI" />
                <Card variant="matrix" padding="sm" className="bg-gray-800/50 max-w-[80%]">
                  <div className="text-sm text-gray-100 whitespace-pre-wrap">
                    {streamingContent}
                    <span className="inline-block w-2 h-4 bg-cyan-400 animate-pulse ml-1" />
                  </div>
                </Card>
              </div>
            )}

            {/* Loading indicator */}
            {isLoading && !streamingContent && (
              <div className="flex gap-3">
                <Avatar size="sm" variant="matrix" fallback="AI" />
                <Card variant="matrix" padding="sm" className="bg-gray-800/50">
                  <Loading variant="dots" color="cyberpunk" text="Thinking..." />
                </Card>
              </div>
            )}

            <div ref={messagesEndRef} />
          </>
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <Sparkles className="w-12 h-12 text-cyan-400/50 mb-4" />
            <h3 className="text-lg font-semibold text-gray-300 mb-2">
              AI Assistant Ready
            </h3>
            <p className="text-gray-400 mb-4 max-w-xs">
              Ask questions about your code, get help debugging, or generate new features.
            </p>
            <Button onClick={createConversation} icon={<MessageSquare size={16} />}>
              Start New Chat
            </Button>
          </div>
        )}
      </div>

      {/* Input area */}
      <div className="border-t border-gray-700/50 p-4 relative">
        {/* Mention suggestions */}
        {showMentions && (
          <div className="absolute bottom-full left-4 right-4 mb-2 max-h-48 overflow-y-auto bg-gray-800 border border-gray-700 rounded-lg shadow-lg z-50">
            {getMentionSuggestions().map((suggestion, index) => (
              <button
                key={`${suggestion.type}-${suggestion.name}-${index}`}
                onClick={() => handleMentionSelect(suggestion)}
                className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-700 transition-colors"
              >
                {suggestion.icon}
                <span className="text-gray-200">{suggestion.name}</span>
                {suggestion.path && (
                  <span className="text-gray-500 text-xs truncate ml-auto">{suggestion.path}</span>
                )}
              </button>
            ))}
            {getMentionSuggestions().length === 0 && (
              <div className="px-3 py-2 text-sm text-gray-400">
                No matches found
              </div>
            )}
          </div>
        )}

        <div className="flex items-end gap-2">
          <div className="flex-1 relative">
            <textarea
              ref={inputRef}
              value={inputMessage}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder="Ask AI about your code... (use @ to mention files)"
              className="w-full bg-gray-800/50 border border-gray-600 rounded-lg px-3 py-2 text-white placeholder:text-gray-400 focus:border-cyan-400 focus:outline-none resize-none min-h-[44px] max-h-32"
              rows={1}
              disabled={isLoading}
              style={{ height: 'auto' }}
              onInput={(e) => {
                const target = e.target as HTMLTextAreaElement
                target.style.height = 'auto'
                target.style.height = Math.min(target.scrollHeight, 128) + 'px'
              }}
            />

            <button
              className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-cyan-400 transition-colors"
              onClick={() => {
                setShowMentions(!showMentions)
                inputRef.current?.focus()
              }}
              title="Mention file or function"
            >
              <AtSign size={16} />
            </button>
          </div>

          <Button
            onClick={() => sendMessage()}
            disabled={!inputMessage.trim() || isLoading}
            loading={isLoading}
            icon={<Send size={16} />}
            variant="primary"
          >
            Send
          </Button>
        </div>

        <div className="flex items-center justify-between mt-2 text-xs text-gray-400">
          <span>Shift+Enter for new line, Enter to send</span>
          <span>@file:name to reference files</span>
        </div>
      </div>
    </div>
  )
}

export default AIChatPanel
