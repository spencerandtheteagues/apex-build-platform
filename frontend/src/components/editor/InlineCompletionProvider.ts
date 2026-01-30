// APEX.BUILD Inline Completion Provider
// Monaco Editor integration for ghost text suggestions (Tab completion)

import * as monaco from 'monaco-editor'
import { aiService, AICompletionSuggestion } from '@/services/aiService'

export interface InlineCompletionConfig {
  enabled: boolean
  debounceMs: number
  triggerCharacters: string[]
  minTriggerLength: number
  maxContextLines: number
  provider: 'claude' | 'gpt4' | 'gemini' | 'auto'
}

const DEFAULT_CONFIG: InlineCompletionConfig = {
  enabled: true,
  debounceMs: 300,
  triggerCharacters: ['.', '(', '{', '[', ':', ' ', '\n'],
  minTriggerLength: 3,
  maxContextLines: 50,
  provider: 'auto',
}

export class InlineCompletionProvider implements monaco.languages.InlineCompletionsProvider {
  private config: InlineCompletionConfig
  private debounceTimer: ReturnType<typeof setTimeout> | null = null
  private currentSuggestion: AICompletionSuggestion | null = null
  private lastRequestId: string | null = null
  private isEnabled: boolean = true

  constructor(config: Partial<InlineCompletionConfig> = {}) {
    this.config = { ...DEFAULT_CONFIG, ...config }
  }

  // Update configuration
  updateConfig(config: Partial<InlineCompletionConfig>): void {
    this.config = { ...this.config, ...config }
  }

  // Enable/disable the provider
  setEnabled(enabled: boolean): void {
    this.isEnabled = enabled
    this.config.enabled = enabled
    if (!enabled) {
      this.clearSuggestion()
    }
  }

  // Clear current suggestion
  clearSuggestion(): void {
    this.currentSuggestion = null
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer)
      this.debounceTimer = null
    }
  }

  // Main provider method called by Monaco
  async provideInlineCompletions(
    model: monaco.editor.ITextModel,
    position: monaco.Position,
    context: monaco.languages.InlineCompletionContext,
    token: monaco.CancellationToken
  ): Promise<monaco.languages.InlineCompletions | undefined> {
    if (!this.config.enabled || !this.isEnabled) {
      return undefined
    }

    // Check if we should trigger completion
    if (!this.shouldTrigger(model, position, context)) {
      return undefined
    }

    // Clear previous timer
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer)
    }

    // Debounce the request
    return new Promise((resolve) => {
      this.debounceTimer = setTimeout(async () => {
        if (token.isCancellationRequested) {
          resolve(undefined)
          return
        }

        try {
          const suggestion = await this.getCompletion(model, position)
          if (token.isCancellationRequested || !suggestion) {
            resolve(undefined)
            return
          }

          this.currentSuggestion = suggestion

          resolve({
            items: [{
              insertText: suggestion.text,
              range: new monaco.Range(
                suggestion.range.startLine,
                suggestion.range.startColumn,
                suggestion.range.endLine,
                suggestion.range.endColumn
              ),
              // Show as ghost text
              command: {
                id: 'apex.acceptInlineCompletion',
                title: 'Accept Inline Completion',
                arguments: [suggestion],
              },
            }],
          })
        } catch (error) {
          console.error('Inline completion error:', error)
          resolve(undefined)
        }
      }, this.config.debounceMs)
    })
  }

  // Check if we should trigger completion
  private shouldTrigger(
    model: monaco.editor.ITextModel,
    position: monaco.Position,
    context: monaco.languages.InlineCompletionContext
  ): boolean {
    // Check if triggered by a specific character
    if (context.triggerKind === monaco.languages.InlineCompletionTriggerKind.Automatic) {
      const lineContent = model.getLineContent(position.lineNumber)
      const charBeforeCursor = lineContent.charAt(position.column - 2)

      // Check minimum trigger length
      const textBeforeCursor = lineContent.substring(0, position.column - 1).trim()
      if (textBeforeCursor.length < this.config.minTriggerLength) {
        return false
      }

      // Check if trigger character
      if (!this.config.triggerCharacters.includes(charBeforeCursor)) {
        // Still allow if we're at the end of a word
        const lastWord = textBeforeCursor.split(/[\s\.\(\)\{\}\[\]:]+/).pop() || ''
        if (lastWord.length < this.config.minTriggerLength) {
          return false
        }
      }
    }

    return true
  }

  // Get completion from AI service
  private async getCompletion(
    model: monaco.editor.ITextModel,
    position: monaco.Position
  ): Promise<AICompletionSuggestion | null> {
    const language = model.getLanguageId()
    const fullContent = model.getValue()
    const offset = model.getOffsetAt(position)

    // Get context around cursor
    const prefix = this.getContext(model, position, 'before')
    const suffix = this.getContext(model, position, 'after')

    // Build prompt from context
    const prompt = this.buildPrompt(prefix, suffix, language)

    // Generate request ID
    const requestId = crypto.randomUUID()
    this.lastRequestId = requestId

    try {
      const suggestion = await aiService.getInlineCompletion({
        prompt,
        code: fullContent,
        language,
        cursorPosition: {
          line: position.lineNumber,
          column: position.column,
        },
        prefix,
        suffix,
        maxTokens: 100,
      })

      // Check if this request is still valid
      if (this.lastRequestId !== requestId) {
        return null
      }

      return suggestion
    } catch (error) {
      console.error('Failed to get inline completion:', error)
      return null
    }
  }

  // Get context lines before or after cursor
  private getContext(
    model: monaco.editor.ITextModel,
    position: monaco.Position,
    direction: 'before' | 'after'
  ): string {
    const lines: string[] = []
    const maxLines = this.config.maxContextLines

    if (direction === 'before') {
      const startLine = Math.max(1, position.lineNumber - maxLines)
      for (let i = startLine; i < position.lineNumber; i++) {
        lines.push(model.getLineContent(i))
      }
      // Add partial current line
      const currentLine = model.getLineContent(position.lineNumber)
      lines.push(currentLine.substring(0, position.column - 1))
    } else {
      // Add remaining current line
      const currentLine = model.getLineContent(position.lineNumber)
      lines.push(currentLine.substring(position.column - 1))
      // Add lines after
      const endLine = Math.min(model.getLineCount(), position.lineNumber + maxLines)
      for (let i = position.lineNumber + 1; i <= endLine; i++) {
        lines.push(model.getLineContent(i))
      }
    }

    return lines.join('\n')
  }

  // Build completion prompt
  private buildPrompt(prefix: string, suffix: string, language: string): string {
    return `Complete the ${language} code at the cursor position. Only provide the completion text, no explanations.

Code before cursor:
\`\`\`${language}
${prefix}
\`\`\`

Code after cursor:
\`\`\`${language}
${suffix}
\`\`\`

Provide a natural continuation of the code:`
  }

  // Handle free completions (required by interface)
  freeInlineCompletions(completions: monaco.languages.InlineCompletions): void {
    // Cleanup if needed
  }
}

// Register the inline completion provider for a language
export function registerInlineCompletionProvider(
  languages: string[],
  config?: Partial<InlineCompletionConfig>
): monaco.IDisposable {
  const provider = new InlineCompletionProvider(config)

  const disposables: monaco.IDisposable[] = []

  for (const language of languages) {
    const disposable = monaco.languages.registerInlineCompletionsProvider(language, provider)
    disposables.push(disposable)
  }

  // Return combined disposable
  return {
    dispose: () => {
      disposables.forEach((d) => d.dispose())
      provider.clearSuggestion()
    },
  }
}

// Create and export default provider instance
export const inlineCompletionProvider = new InlineCompletionProvider()

// Supported languages for inline completion
export const SUPPORTED_LANGUAGES = [
  'javascript',
  'typescript',
  'javascriptreact',
  'typescriptreact',
  'python',
  'go',
  'rust',
  'java',
  'cpp',
  'c',
  'csharp',
  'php',
  'ruby',
  'swift',
  'kotlin',
  'html',
  'css',
  'scss',
  'json',
  'yaml',
  'markdown',
  'sql',
]

export default InlineCompletionProvider
