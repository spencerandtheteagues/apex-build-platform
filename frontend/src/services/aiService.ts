// APEX.BUILD Advanced AI Service
// Multi-provider streaming support with intelligent routing

import { AICapability, AIProvider, AIMessage } from '@/types'

export interface AIStreamChunk {
  type: 'content' | 'error' | 'done' | 'usage'
  content?: string
  error?: string
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
    cost: number
  }
  provider?: AIProvider
}

export interface AICompletionRequest {
  prompt: string
  code: string
  language: string
  cursorPosition: { line: number; column: number }
  prefix: string
  suffix: string
  maxTokens?: number
}

export interface AICompletionSuggestion {
  text: string
  range: {
    startLine: number
    startColumn: number
    endLine: number
    endColumn: number
  }
  confidence: number
}

export interface AIChatContext {
  projectId?: number
  fileId?: number
  fileName?: string
  language?: string
  currentCode?: string
  selectedCode?: string
  cursorPosition?: { line: number; column: number }
  referencedFiles?: Array<{ path: string; content: string }>
  conversationHistory?: AIMessage[]
}

export interface AIGenerationOptions {
  capability: AICapability
  prompt: string
  code?: string
  language?: string
  context?: AIChatContext
  provider?: AIProvider | 'auto'
  temperature?: number
  maxTokens?: number
  stream?: boolean
}

export type StreamCallback = (chunk: AIStreamChunk) => void

// Get API URL from environment or use default
const getApiUrl = (): string => {
  if (import.meta.env.VITE_API_URL) {
    return import.meta.env.VITE_API_URL
  }
  return '/api/v1'
}

class AIService {
  private baseURL: string
  private abortControllers: Map<string, AbortController> = new Map()

  constructor(baseURL: string = getApiUrl()) {
    this.baseURL = baseURL
  }

  private getAuthToken(): string | null {
    return localStorage.getItem('apex_access_token')
  }

  // Generate AI completion with optional streaming
  async generate(
    options: AIGenerationOptions,
    onStream?: StreamCallback
  ): Promise<string> {
    const {
      capability,
      prompt,
      code,
      language,
      context,
      provider = 'auto',
      temperature = 0.7,
      maxTokens = 4000,
      stream = false
    } = options

    const requestId = crypto.randomUUID()
    const abortController = new AbortController()
    this.abortControllers.set(requestId, abortController)

    try {
      if (stream && onStream) {
        return await this.streamGenerate(
          requestId,
          { capability, prompt, code, language, context, provider, temperature, maxTokens },
          onStream,
          abortController.signal
        )
      } else {
        return await this.normalGenerate(
          { capability, prompt, code, language, context, provider, temperature, maxTokens },
          abortController.signal
        )
      }
    } finally {
      this.abortControllers.delete(requestId)
    }
  }

  // Streaming generation using Server-Sent Events
  private async streamGenerate(
    requestId: string,
    options: Omit<AIGenerationOptions, 'stream'>,
    onStream: StreamCallback,
    signal: AbortSignal
  ): Promise<string> {
    const token = this.getAuthToken()

    const response = await fetch(`${this.baseURL}/ai/stream`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': token ? `Bearer ${token}` : '',
        'Accept': 'text/event-stream',
      },
      body: JSON.stringify({
        capability: options.capability,
        prompt: options.prompt,
        code: options.code,
        language: options.language,
        context: options.context,
        provider: options.provider,
        temperature: options.temperature,
        max_tokens: options.maxTokens,
        stream: true,
      }),
      signal,
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'AI generation failed')
    }

    const reader = response.body?.getReader()
    if (!reader) {
      throw new Error('No response body')
    }

    const decoder = new TextDecoder()
    let fullContent = ''
    let buffer = '' // Buffer to handle incomplete lines between chunks

    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        const chunk = decoder.decode(value, { stream: true })
        buffer += chunk
        const lines = buffer.split('\n')
        buffer = lines.pop() || '' // Keep incomplete line for next chunk

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') {
              onStream({ type: 'done' })
              break
            }

            try {
              const parsed = JSON.parse(data) as AIStreamChunk
              if (parsed.type === 'content' && parsed.content) {
                fullContent += parsed.content
              }
              onStream(parsed)
            } catch {
              // Ignore parse errors for incomplete chunks
            }
          }
        }
      }
    } finally {
      reader.releaseLock()
    }

    return fullContent
  }

  // Normal (non-streaming) generation
  private async normalGenerate(
    options: Omit<AIGenerationOptions, 'stream'>,
    signal: AbortSignal
  ): Promise<string> {
    const token = this.getAuthToken()

    const response = await fetch(`${this.baseURL}/ai/generate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': token ? `Bearer ${token}` : '',
      },
      body: JSON.stringify({
        capability: options.capability,
        prompt: options.prompt,
        code: options.code,
        language: options.language,
        context: options.context,
        provider: options.provider,
        temperature: options.temperature,
        max_tokens: options.maxTokens,
      }),
      signal,
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'AI generation failed')
    }

    const data = await response.json()
    return data.data?.content || data.content || ''
  }

  // Get inline code completions (for ghost text)
  async getInlineCompletion(
    request: AICompletionRequest,
    abortSignal?: AbortSignal
  ): Promise<AICompletionSuggestion | null> {
    const token = this.getAuthToken()

    try {
      const response = await fetch(`${this.baseURL}/ai/complete`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': token ? `Bearer ${token}` : '',
        },
        body: JSON.stringify({
          capability: 'code_completion',
          prompt: request.prompt,
          code: request.code,
          language: request.language,
          context: {
            prefix: request.prefix,
            suffix: request.suffix,
            cursor_line: request.cursorPosition.line,
            cursor_column: request.cursorPosition.column,
          },
          max_tokens: request.maxTokens || 150,
          temperature: 0.2, // Lower temperature for completions
        }),
        signal: abortSignal,
      })

      if (!response.ok) {
        return null
      }

      const data = await response.json()
      const completion = data.data?.content || data.content

      if (!completion) {
        return null
      }

      return {
        text: completion,
        range: {
          startLine: request.cursorPosition.line,
          startColumn: request.cursorPosition.column,
          endLine: request.cursorPosition.line,
          endColumn: request.cursorPosition.column,
        },
        confidence: data.confidence || 0.8,
      }
    } catch (error: unknown) {
      // Don't log abort errors as they are expected
      if (error instanceof Error && error.name !== 'AbortError') {
        console.error('Inline completion error:', error)
      }
      return null
    }
  }

  // Generate tests for code
  async generateTests(
    code: string,
    language: string,
    framework?: string
  ): Promise<string> {
    return this.generate({
      capability: 'testing',
      prompt: `Generate comprehensive unit tests for the following ${language} code${framework ? ` using ${framework}` : ''}. Include edge cases and error scenarios.`,
      code,
      language,
      temperature: 0.3,
    })
  }

  // Generate documentation
  async generateDocumentation(
    code: string,
    language: string,
    style?: 'jsdoc' | 'docstring' | 'markdown'
  ): Promise<string> {
    const styleInstructions = {
      jsdoc: 'Use JSDoc format with @param, @returns, and @example tags.',
      docstring: 'Use Python docstring format with Args, Returns, and Examples sections.',
      markdown: 'Generate markdown documentation with function signatures, descriptions, and examples.',
    }

    return this.generate({
      capability: 'documentation',
      prompt: `Generate comprehensive documentation for the following ${language} code. ${style ? styleInstructions[style] : ''}`,
      code,
      language,
      temperature: 0.3,
    })
  }

  // Code review
  async reviewCode(
    code: string,
    language: string,
    options?: {
      checkSecurity?: boolean
      checkPerformance?: boolean
      checkStyle?: boolean
    }
  ): Promise<string> {
    const checks: string[] = []
    if (options?.checkSecurity) checks.push('security vulnerabilities')
    if (options?.checkPerformance) checks.push('performance issues')
    if (options?.checkStyle) checks.push('code style and best practices')

    const focus = checks.length > 0
      ? `Focus on: ${checks.join(', ')}.`
      : 'Check for bugs, security issues, performance problems, and best practices.'

    return this.generate({
      capability: 'code_review',
      prompt: `Review the following ${language} code. ${focus} Provide specific, actionable feedback.`,
      code,
      language,
      temperature: 0.4,
    })
  }

  // Explain code
  async explainCode(
    code: string,
    language: string,
    level?: 'beginner' | 'intermediate' | 'expert'
  ): Promise<string> {
    const levelInstructions = {
      beginner: 'Explain in simple terms, avoiding jargon. Include analogies where helpful.',
      intermediate: 'Provide a balanced explanation with some technical details.',
      expert: 'Provide a detailed technical explanation including implementation details.',
    }

    return this.generate({
      capability: 'explanation',
      prompt: `Explain the following ${language} code. ${level ? levelInstructions[level] : ''}`,
      code,
      language,
      temperature: 0.5,
    })
  }

  // Debug code
  async debugCode(
    code: string,
    language: string,
    error?: string
  ): Promise<string> {
    const errorContext = error
      ? `The following error was encountered: ${error}`
      : 'Identify potential bugs and issues.'

    return this.generate({
      capability: 'debugging',
      prompt: `Debug the following ${language} code. ${errorContext} Explain the issue and provide a fix.`,
      code,
      language,
      temperature: 0.3,
    })
  }

  // Refactor code
  async refactorCode(
    code: string,
    language: string,
    instructions?: string
  ): Promise<string> {
    return this.generate({
      capability: 'refactoring',
      prompt: `Refactor the following ${language} code. ${instructions || 'Improve readability, maintainability, and follow best practices.'}`,
      code,
      language,
      temperature: 0.4,
    })
  }

  // Cancel ongoing request
  cancelRequest(requestId: string): boolean {
    const controller = this.abortControllers.get(requestId)
    if (controller) {
      controller.abort()
      this.abortControllers.delete(requestId)
      return true
    }
    return false
  }

  // Cancel all ongoing requests
  cancelAllRequests(): void {
    this.abortControllers.forEach((controller) => {
      controller.abort()
    })
    this.abortControllers.clear()
  }

  // Parse @mentions from text
  parseMentions(text: string): Array<{ type: 'file' | 'function' | 'symbol'; name: string; start: number; end: number }> {
    const mentions: Array<{ type: 'file' | 'function' | 'symbol'; name: string; start: number; end: number }> = []

    // Match @file:path/to/file
    const fileRegex = /@file:([^\s]+)/g
    let match
    while ((match = fileRegex.exec(text)) !== null) {
      mentions.push({
        type: 'file',
        name: match[1],
        start: match.index,
        end: match.index + match[0].length,
      })
    }

    // Match @function:functionName
    const funcRegex = /@function:([^\s]+)/g
    while ((match = funcRegex.exec(text)) !== null) {
      mentions.push({
        type: 'function',
        name: match[1],
        start: match.index,
        end: match.index + match[0].length,
      })
    }

    // Match @symbol:symbolName
    const symbolRegex = /@symbol:([^\s]+)/g
    while ((match = symbolRegex.exec(text)) !== null) {
      mentions.push({
        type: 'symbol',
        name: match[1],
        start: match.index,
        end: match.index + match[0].length,
      })
    }

    return mentions.sort((a, b) => a.start - b.start)
  }
}

// Create singleton instance
export const aiService = new AIService()
export default aiService
