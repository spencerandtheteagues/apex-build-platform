// APEX.BUILD Multi-AI Assistant Interface
// Cyberpunk AI chat interface with provider switching

import React, { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { useStore } from '@/hooks/useStore'
import {
  AIMessage,
  AIProvider,
  AICapability,
  AIConversation,
  User
} from '@/types'
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Badge,
  AIProviderBadge,
  Avatar,
  Loading,
  Input
} from '@/components/ui'
import {
  Send,
  Sparkles,
  Code,
  Bug,
  RefreshCw,
  FileText,
  MessageSquare,
  Zap,
  Brain,
  Cpu,
  Eye,
  Copy,
  ThumbsUp,
  ThumbsDown,
  MoreVertical
} from 'lucide-react'
import { formatCost, formatRelativeTime, generateId } from '@/lib/utils'

export interface AIAssistantProps {
  className?: string
  projectId?: number
  fileId?: number
  defaultProvider?: AIProvider | 'auto'
  onCodeInsert?: (code: string) => void
}

const AICapabilityIcons = {
  code_generation: Code,
  code_completion: Sparkles,
  debugging: Bug,
  refactoring: RefreshCw,
  explanation: MessageSquare,
  code_review: Eye,
  testing: FileText,
  documentation: FileText,
}

const ProviderIcons = {
  claude: Brain,
  gpt4: Cpu,
  gemini: Zap,
}

export const AIAssistant: React.FC<AIAssistantProps> = ({
  className,
  projectId,
  fileId,
  defaultProvider = 'auto',
  onCodeInsert,
}) => {
  const [conversations, setConversations] = useState<AIConversation[]>([])
  const [activeConversation, setActiveConversation] = useState<string | null>(null)
  const [inputMessage, setInputMessage] = useState('')
  const [selectedProvider, setSelectedProvider] = useState<AIProvider | 'auto'>(defaultProvider)
  const [selectedCapability, setSelectedCapability] = useState<AICapability>('code_generation')
  const [isLoading, setIsLoading] = useState(false)
  const [contextCode, setContextCode] = useState('')

  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const { user, currentProject, apiService } = useStore()

  // Auto-scroll to bottom of messages
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottom()
  }, [conversations])

  // Get current conversation
  const currentConversation = conversations.find(c => c.id === activeConversation)

  // Create new conversation
  const createConversation = () => {
    const newConversation: AIConversation = {
      id: generateId(),
      messages: [],
      project_id: projectId,
      file_id: fileId,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }

    setConversations(prev => [newConversation, ...prev])
    setActiveConversation(newConversation.id)
  }

  // Send message to AI
  const sendMessage = async () => {
    if (!inputMessage.trim() || isLoading) return

    const conversationId = activeConversation || (() => {
      createConversation()
      return conversations[0]?.id
    })()

    const userMessage: AIMessage = {
      id: generateId(),
      type: 'user',
      content: inputMessage,
      timestamp: new Date().toISOString(),
    }

    // Add user message to conversation
    setConversations(prev => prev.map(conv =>
      conv.id === conversationId
        ? { ...conv, messages: [...conv.messages, userMessage] }
        : conv
    ))

    setInputMessage('')
    setIsLoading(true)

    try {
      // Call AI API
      const response = await apiService.generateAI({
        capability: selectedCapability,
        prompt: inputMessage,
        code: contextCode,
        language: getFileLanguage(),
        context: {
          project_id: projectId,
          file_id: fileId,
          conversation_id: conversationId,
        },
        max_tokens: 4000,
        temperature: 0.7,
      })

      const aiMessage: AIMessage = {
        id: generateId(),
        type: 'assistant',
        content: response.content,
        provider: response.provider,
        capability: selectedCapability,
        code: extractCodeFromResponse(response.content),
        timestamp: response.created_at,
        usage: response.usage ? {
          tokens: response.usage.total_tokens,
          cost: response.usage.cost,
          duration: response.duration,
        } : undefined,
      }

      // Add AI response to conversation
      setConversations(prev => prev.map(conv =>
        conv.id === conversationId
          ? {
              ...conv,
              messages: [...conv.messages, aiMessage],
              updated_at: new Date().toISOString()
            }
          : conv
      ))

    } catch (error) {
      console.error('AI generation error:', error)

      const errorMessage: AIMessage = {
        id: generateId(),
        type: 'system',
        content: `Error: ${error instanceof Error ? error.message : 'Failed to generate AI response'}`,
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
    }
  }

  // Extract code blocks from AI response
  const extractCodeFromResponse = (content: string): string | undefined => {
    const codeBlockRegex = /```[\w]*\n([\s\S]*?)\n```/g
    const matches = content.match(codeBlockRegex)
    if (matches && matches.length > 0) {
      return matches[0].replace(/```[\w]*\n|```$/g, '').trim()
    }
    return undefined
  }

  // Get file language from current context
  const getFileLanguage = (): string => {
    // This would be determined from the active file in the editor
    return 'javascript' // Default fallback
  }

  // Handle keyboard shortcuts
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      if (e.shiftKey) {
        // Allow new line
        return
      } else {
        e.preventDefault()
        sendMessage()
      }
    }
  }

  // Copy code to clipboard
  const copyCode = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code)
      // Show success notification
    } catch (error) {
      console.error('Failed to copy code:', error)
    }
  }

  // Insert code into editor
  const insertCode = (code: string) => {
    onCodeInsert?.(code)
  }

  // Render message
  const renderMessage = (message: AIMessage) => {
    const isUser = message.type === 'user'
    const isSystem = message.type === 'system'
    const isAI = message.type === 'assistant'

    return (
      <div
        key={message.id}
        className={cn(
          'flex gap-3 p-4',
          isUser && 'flex-row-reverse',
          isSystem && 'justify-center'
        )}
      >
        {/* Avatar */}
        {!isSystem && (
          <Avatar
            size="sm"
            variant={isUser ? 'cyberpunk' : 'matrix'}
            fallback={isUser ? user?.username : 'AI'}
            src={isUser ? user?.avatar_url : undefined}
          />
        )}

        {/* Message content */}
        <div className={cn(
          'flex flex-col gap-2 max-w-[80%]',
          isUser && 'items-end',
          isSystem && 'items-center'
        )}>
          {/* Message header */}
          {!isSystem && (
            <div className={cn(
              'flex items-center gap-2 text-xs text-gray-400',
              isUser && 'flex-row-reverse'
            )}>
              <span>{isUser ? user?.username : 'AI Assistant'}</span>
              {isAI && message.provider && (
                <AIProviderBadge provider={message.provider} size="xs" />
              )}
              {message.capability && (
                <Badge variant="outline" size="xs">
                  {message.capability.replace('_', ' ')}
                </Badge>
              )}
              <span>{formatRelativeTime(message.timestamp)}</span>
            </div>
          )}

          {/* Message bubble */}
          <Card
            variant={
              isSystem ? 'neutral' :
              isUser ? 'cyberpunk' :
              message.error ? 'error' : 'matrix'
            }
            padding="sm"
            className={cn(
              'max-w-full',
              isUser && 'bg-cyan-900/30',
              isAI && 'bg-gray-800/50',
              isSystem && 'bg-yellow-900/20 border-yellow-500/30'
            )}
          >
            <div className="prose prose-invert prose-sm max-w-none">
              {/* Regular content */}
              <div className="whitespace-pre-wrap text-sm text-gray-100">
                {message.content}
              </div>

              {/* Code blocks */}
              {message.code && (
                <div className="mt-3 bg-gray-900/70 rounded border border-gray-700/50 overflow-hidden">
                  <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 border-b border-gray-700/50">
                    <span className="text-xs text-gray-400">Generated Code</span>
                    <div className="flex items-center gap-1">
                      <Button
                        size="xs"
                        variant="ghost"
                        onClick={() => copyCode(message.code!)}
                        icon={<Copy size={12} />}
                      />
                      {onCodeInsert && (
                        <Button
                          size="xs"
                          variant="ghost"
                          onClick={() => insertCode(message.code!)}
                          icon={<Code size={12} />}
                        >
                          Insert
                        </Button>
                      )}
                    </div>
                  </div>
                  <pre className="p-3 text-xs text-gray-100 overflow-x-auto">
                    <code>{message.code}</code>
                  </pre>
                </div>
              )}

              {/* Usage stats */}
              {message.usage && (
                <div className="mt-2 flex items-center gap-3 text-xs text-gray-400">
                  <span>{message.usage.tokens} tokens</span>
                  <span>{formatCost(message.usage.cost)}</span>
                  <span>{message.usage.duration}ms</span>
                </div>
              )}
            </div>
          </Card>

          {/* Message actions */}
          {isAI && !message.error && (
            <div className="flex items-center gap-1">
              <Button size="xs" variant="ghost" icon={<ThumbsUp size={12} />} />
              <Button size="xs" variant="ghost" icon={<ThumbsDown size={12} />} />
              <Button size="xs" variant="ghost" icon={<MoreVertical size={12} />} />
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col h-full bg-gray-900/50', className)}>
      {/* Header */}
      <CardHeader className="border-b border-gray-700/50">
        <div className="flex items-center justify-between">
          <CardTitle className="text-lg flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-cyan-400" />
            AI Assistant
          </CardTitle>

          <div className="flex items-center gap-2">
            {/* Provider selector */}
            <select
              value={selectedProvider}
              onChange={(e) => setSelectedProvider(e.target.value as AIProvider | 'auto')}
              className="bg-gray-800 border border-gray-600 rounded px-2 py-1 text-xs text-white focus:border-cyan-400 focus:outline-none"
            >
              <option value="auto">Auto-Select</option>
              <option value="claude">Claude</option>
              <option value="gpt4">GPT-4</option>
              <option value="gemini">Gemini</option>
            </select>

            <Button
              size="sm"
              variant="ghost"
              onClick={createConversation}
              icon={<MessageSquare size={14} />}
            >
              New Chat
            </Button>
          </div>
        </div>
      </CardHeader>

      {/* Messages */}
      <CardContent className="flex-1 overflow-y-auto p-0">
        {currentConversation ? (
          <div className="space-y-1">
            {currentConversation.messages.map(renderMessage)}
            {isLoading && (
              <div className="flex justify-center p-4">
                <Loading variant="dots" color="cyberpunk" text="AI thinking..." />
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-center p-8">
            <Sparkles className="w-12 h-12 text-cyan-400/50 mb-4" />
            <h3 className="text-lg font-semibold text-gray-300 mb-2">
              Welcome to APEX.BUILD AI Assistant
            </h3>
            <p className="text-gray-400 mb-4">
              Start a conversation to get help with coding, debugging, and more.
            </p>
            <Button onClick={createConversation} icon={<MessageSquare size={16} />}>
              Start New Conversation
            </Button>
          </div>
        )}
      </CardContent>

      {/* Input area */}
      {currentConversation && (
        <div className="border-t border-gray-700/50 p-4">
          {/* Capability selector */}
          <div className="flex items-center gap-2 mb-3">
            {Object.entries(AICapabilityIcons).map(([capability, IconComponent]) => (
              <Button
                key={capability}
                size="xs"
                variant={selectedCapability === capability ? 'primary' : 'ghost'}
                onClick={() => setSelectedCapability(capability as AICapability)}
                icon={<IconComponent size={12} />}
              >
                {capability.replace('_', ' ')}
              </Button>
            ))}
          </div>

          {/* Input */}
          <div className="flex items-end gap-2">
            <div className="flex-1">
              <textarea
                ref={inputRef}
                value={inputMessage}
                onChange={(e) => setInputMessage(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Ask AI to help with your code..."
                className="w-full bg-gray-800/50 border border-gray-600 rounded-lg px-3 py-2 text-white placeholder:text-gray-400 focus:border-cyan-400 focus:outline-none resize-none"
                rows={2}
                disabled={isLoading}
              />
            </div>
            <Button
              onClick={sendMessage}
              disabled={!inputMessage.trim() || isLoading}
              loading={isLoading}
              icon={<Send size={16} />}
              variant="primary"
            >
              Send
            </Button>
          </div>

          <div className="flex items-center justify-between mt-2 text-xs text-gray-400">
            <span>Press Shift+Enter for new line, Enter to send</span>
            <span>Provider: {selectedProvider === 'auto' ? 'Auto-Select' : selectedProvider}</span>
          </div>
        </div>
      )}
    </div>
  )
}

export default AIAssistant