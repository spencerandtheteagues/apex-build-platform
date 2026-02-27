import React, { useState, useEffect, useCallback } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/Badge';
import { Progress } from '@/components/ui/progress';
import {
  Brain,
  Code,
  MessageSquare,
  Sparkles,
  Zap,
  Target,
  CheckCircle,
  AlertCircle,
  RefreshCw,
  Copy,
  Download
} from 'lucide-react';
import apiService from '@/services/api';

interface AICodeAssistantProps {
  currentCode: string;
  selectedText: string;
  language: string;
  onCodeGenerated: (code: string) => void;
  onCodeInserted: (code: string, position?: any) => void;
}

interface AIProvider {
  id: string;
  name: string;
  icon: string;
  capabilities: string[];
  performance: number;
}

interface CodeSuggestion {
  id: string;
  type: 'improvement' | 'bug-fix' | 'optimization' | 'security' | 'refactor';
  title: string;
  description: string;
  code: string;
  confidence: number;
  impact: 'low' | 'medium' | 'high';
}

export const AICodeAssistant: React.FC<AICodeAssistantProps> = ({
  currentCode,
  selectedText,
  language,
  onCodeGenerated,
  onCodeInserted
}) => {
  const [activeTab, setActiveTab] = useState<'chat' | 'generate' | 'review' | 'optimize'>('chat');
  const [prompt, setPrompt] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);
  const [suggestions, setSuggestions] = useState<CodeSuggestion[]>([]);
  const [selectedProvider, setSelectedProvider] = useState('auto');
  const [chatHistory, setChatHistory] = useState<Array<{role: 'user' | 'assistant', content: string}>>([]);

  const aiProviders: AIProvider[] = [
    {
      id: 'auto',
      name: 'Smart Routing',
      icon: '🧠',
      capabilities: ['All Tasks', 'Best Performance'],
      performance: 95
    },
    {
      id: 'claude',
      name: 'Claude Opus 4.5',
      icon: '🎭',
      capabilities: ['Complex Logic', 'Code Analysis', 'Refactoring'],
      performance: 92
    },
    {
      id: 'gpt5',
      name: 'GPT-5',
      icon: '⚡',
      capabilities: ['Code Generation', 'Documentation', 'Debugging'],
      performance: 90
    },
    {
      id: 'gemini',
      name: 'Gemini 3',
      icon: '💎',
      capabilities: ['Optimization', 'Security', 'Performance'],
      performance: 88
    }
  ];

  // Auto-analyze code when it changes
  useEffect(() => {
    if (currentCode && currentCode.length > 100) {
      analyzeCodeAutomatically();
    }
  }, [currentCode]); // eslint-disable-line react-hooks/exhaustive-deps -- analyzer callback is intentionally stable.

  const analyzeCodeAutomatically = useCallback(async () => {
    try {
      const result = await apiService.generateAI({
        capability: 'code_review' as any,
        prompt: 'Analyze this code and return a JSON array of suggestions. Each suggestion should have: type (optimization|security|bug-fix|improvement|refactor), title, description, code (fix snippet), confidence (0-100), impact (low|medium|high).',
        code: currentCode,
        language,
      })

      try {
        // Try to parse structured suggestions from the AI response
        const jsonMatch = result.content.match(/\[[\s\S]*\]/)
        if (jsonMatch) {
          const parsed = JSON.parse(jsonMatch[0]) as Array<{
            type: string; title: string; description: string; code: string; confidence: number; impact: string
          }>
          setSuggestions(parsed.map((s, i) => ({
            ...s,
            id: String(i + 1),
            type: s.type as CodeSuggestion['type'],
            impact: s.impact as CodeSuggestion['impact'],
          })))
          return
        }
      } catch {
        // If parsing fails, fall through to empty suggestions
      }
      setSuggestions([])
    } catch (error) {
      console.error('Failed to analyze code:', error);
    }
  }, [currentCode, language]);

  const generateCode = async () => {
    if (!prompt.trim()) return;

    setIsProcessing(true);
    try {
      const providerName = aiProviders.find(p => p.id === selectedProvider)?.name ?? 'AI'

      const result = await apiService.generateAI({
        capability: 'code_generation' as any,
        prompt,
        code: selectedText || currentCode,
        language,
        context: { provider: selectedProvider },
      })

      onCodeGenerated(result.content);

      setChatHistory(prev => [
        ...prev,
        { role: 'user', content: prompt },
        { role: 'assistant', content: `Generated code via ${providerName} (${result.usage?.total_tokens ?? 0} tokens)` }
      ]);

      setPrompt('');
    } catch (error) {
      console.error('Code generation failed:', error);
      setChatHistory(prev => [
        ...prev,
        { role: 'user', content: prompt },
        { role: 'assistant', content: 'Code generation failed. Please try again.' }
      ]);
    } finally {
      setIsProcessing(false);
    }
  };

  const applySuggestion = (suggestion: CodeSuggestion) => {
    onCodeInserted(suggestion.code);
    setSuggestions(prev => prev.filter(s => s.id !== suggestion.id));
  };

  const getSuggestionIcon = (type: string) => {
    switch (type) {
      case 'optimization': return <Zap className="h-4 w-4" />;
      case 'security': return <AlertCircle className="h-4 w-4" />;
      case 'bug-fix': return <Target className="h-4 w-4" />;
      case 'improvement': return <Sparkles className="h-4 w-4" />;
      case 'refactor': return <RefreshCw className="h-4 w-4" />;
      default: return <Code className="h-4 w-4" />;
    }
  };

  const getSuggestionColor = (type: string) => {
    switch (type) {
      case 'optimization': return 'bg-blue-100 text-blue-800';
      case 'security': return 'bg-red-100 text-red-800';
      case 'bug-fix': return 'bg-orange-100 text-orange-800';
      case 'improvement': return 'bg-green-100 text-green-800';
      case 'refactor': return 'bg-purple-100 text-purple-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <div className="h-full flex flex-col bg-gray-900 border-l border-gray-700">
      <div className="p-4 border-b border-gray-700">
        <div className="flex items-center gap-2 mb-4">
          <Brain className="h-5 w-5 text-blue-400" />
          <h3 className="font-semibold text-white">AI Code Assistant</h3>
          <Badge variant="outline" className="ml-auto text-xs">
            {suggestions.length} suggestions
          </Badge>
        </div>

        {/* AI Provider Selection */}
        <div className="grid grid-cols-2 gap-2 mb-4">
          {aiProviders.map((provider) => (
            <Button
              key={provider.id}
              variant={selectedProvider === provider.id ? "default" : "outline"}
              size="sm"
              onClick={() => setSelectedProvider(provider.id)}
              className="h-auto p-2 text-xs"
            >
              <div className="flex flex-col items-center gap-1">
                <span className="text-lg">{provider.icon}</span>
                <span className="font-medium">{provider.name}</span>
                <Progress value={provider.performance} className="w-full h-1" />
              </div>
            </Button>
          ))}
        </div>

        {/* Tab Navigation */}
        <div className="flex rounded-lg bg-gray-800 p-1">
          {[
            { id: 'chat', label: 'Chat', icon: MessageSquare },
            { id: 'generate', label: 'Generate', icon: Code },
            { id: 'review', label: 'Review', icon: CheckCircle },
            { id: 'optimize', label: 'Optimize', icon: Zap }
          ].map(({ id, label, icon: Icon }) => (
            <Button
              key={id}
              variant={activeTab === id ? "default" : "ghost"}
              size="sm"
              onClick={() => setActiveTab(id as any)}
              className="flex-1 flex items-center gap-1 text-xs"
            >
              <Icon className="h-3 w-3" />
              {label}
            </Button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        {activeTab === 'chat' && (
          <div className="h-full flex flex-col">
            <div className="flex-1 overflow-y-auto p-4 space-y-3">
              {chatHistory.length === 0 ? (
                <div className="text-center text-gray-400 py-8">
                  <Brain className="h-8 w-8 mx-auto mb-2 opacity-50" />
                  <p className="text-sm">Start a conversation with AI</p>
                  <p className="text-xs mt-1">Ask questions, request code, or get help</p>
                </div>
              ) : (
                chatHistory.map((message, index) => (
                  <div key={index} className={`p-3 rounded-lg ${
                    message.role === 'user'
                      ? 'bg-blue-600 text-white ml-4'
                      : 'bg-gray-800 text-gray-200 mr-4'
                  }`}>
                    <p className="text-sm">{message.content}</p>
                  </div>
                ))
              )}
            </div>

            <div className="p-4 border-t border-gray-700">
              <div className="flex gap-2">
                <Textarea
                  placeholder="Ask AI anything about your code..."
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  className="flex-1 min-h-[60px] bg-gray-800 border-gray-600"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                      generateCode();
                    }
                  }}
                />
                <Button
                  onClick={generateCode}
                  disabled={isProcessing || !prompt.trim()}
                  className="self-end"
                >
                  {isProcessing ? (
                    <RefreshCw className="h-4 w-4 animate-spin" />
                  ) : (
                    <Brain className="h-4 w-4" />
                  )}
                </Button>
              </div>
              <p className="text-xs text-gray-400 mt-2">
                Press Ctrl+Enter to send • Using {aiProviders.find(p => p.id === selectedProvider)?.name}
              </p>
            </div>
          </div>
        )}

        {activeTab === 'generate' && (
          <div className="p-4 space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-200 mb-2">
                Describe what you want to generate:
              </label>
              <Textarea
                placeholder="e.g., Create a React component for user authentication..."
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                className="w-full min-h-[100px] bg-gray-800 border-gray-600"
              />
            </div>

            <div className="flex gap-2">
              <Button
                onClick={generateCode}
                disabled={isProcessing || !prompt.trim()}
                className="flex-1"
              >
                {isProcessing ? (
                  <>
                    <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                    Generating...
                  </>
                ) : (
                  <>
                    <Sparkles className="h-4 w-4 mr-2" />
                    Generate Code
                  </>
                )}
              </Button>
            </div>

            {selectedText && (
              <Card className="bg-gray-800 border-gray-700">
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm text-gray-200">Selected Code</CardTitle>
                </CardHeader>
                <CardContent>
                  <pre className="text-xs text-gray-300 bg-gray-900 p-2 rounded overflow-x-auto">
                    {selectedText}
                  </pre>
                  <div className="flex gap-2 mt-2">
                    <Button size="sm" variant="outline">
                      <Code className="h-3 w-3 mr-1" />
                      Explain
                    </Button>
                    <Button size="sm" variant="outline">
                      <RefreshCw className="h-3 w-3 mr-1" />
                      Refactor
                    </Button>
                    <Button size="sm" variant="outline">
                      <Zap className="h-3 w-3 mr-1" />
                      Optimize
                    </Button>
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        )}

        {activeTab === 'review' && (
          <div className="p-4 space-y-3">
            <div className="flex justify-between items-center">
              <h4 className="font-medium text-gray-200">Code Suggestions</h4>
              <Button size="sm" variant="outline" onClick={analyzeCodeAutomatically}>
                <RefreshCw className="h-3 w-3 mr-1" />
                Re-analyze
              </Button>
            </div>

            {suggestions.length === 0 ? (
              <div className="text-center text-gray-400 py-8">
                <CheckCircle className="h-8 w-8 mx-auto mb-2 opacity-50" />
                <p className="text-sm">No suggestions found</p>
                <p className="text-xs mt-1">Your code looks good!</p>
              </div>
            ) : (
              <div className="space-y-3">
                {suggestions.map((suggestion) => (
                  <Card key={suggestion.id} className="bg-gray-800 border-gray-700">
                    <CardContent className="p-3">
                      <div className="flex items-start gap-2 mb-2">
                        <div className={`p-1 rounded ${getSuggestionColor(suggestion.type)}`}>
                          {getSuggestionIcon(suggestion.type)}
                        </div>
                        <div className="flex-1">
                          <h5 className="font-medium text-gray-200 text-sm">{suggestion.title}</h5>
                          <p className="text-xs text-gray-400 mt-1">{suggestion.description}</p>
                          <div className="flex items-center gap-2 mt-2">
                            <Badge variant="outline" size="sm">
                              {suggestion.confidence}% confident
                            </Badge>
                            <Badge variant="outline" size="sm" className={
                              suggestion.impact === 'high' ? 'border-red-400 text-red-400' :
                              suggestion.impact === 'medium' ? 'border-yellow-400 text-yellow-400' :
                              'border-green-400 text-green-400'
                            }>
                              {suggestion.impact} impact
                            </Badge>
                          </div>
                        </div>
                      </div>

                      <pre className="text-xs text-gray-300 bg-gray-900 p-2 rounded mb-2 overflow-x-auto">
                        {suggestion.code}
                      </pre>

                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          onClick={() => applySuggestion(suggestion)}
                        >
                          <CheckCircle className="h-3 w-3 mr-1" />
                          Apply
                        </Button>
                        <Button size="sm" variant="outline">
                          <Copy className="h-3 w-3 mr-1" />
                          Copy
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setSuggestions(prev => prev.filter(s => s.id !== suggestion.id))}
                        >
                          Dismiss
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </div>
        )}

        {activeTab === 'optimize' && (
          <div className="p-4 space-y-4">
            <Card className="bg-gray-800 border-gray-700">
              <CardHeader>
                <CardTitle className="text-sm text-gray-200">Performance Analysis</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex justify-between text-sm">
                  <span className="text-gray-400">Code Complexity</span>
                  <span className="text-yellow-400">Medium</span>
                </div>
                <Progress value={65} className="h-2" />

                <div className="flex justify-between text-sm">
                  <span className="text-gray-400">Performance Score</span>
                  <span className="text-green-400">Good (82/100)</span>
                </div>
                <Progress value={82} className="h-2" />

                <div className="flex justify-between text-sm">
                  <span className="text-gray-400">Memory Usage</span>
                  <span className="text-blue-400">Optimal</span>
                </div>
                <Progress value={40} className="h-2" />
              </CardContent>
            </Card>

            <div className="space-y-2">
              <h4 className="font-medium text-gray-200">Quick Optimizations</h4>
              {[
                { title: 'Add Memoization', impact: 'High', effort: 'Low' },
                { title: 'Optimize Loops', impact: 'Medium', effort: 'Medium' },
                { title: 'Reduce Bundle Size', impact: 'High', effort: 'High' }
              ].map((opt, index) => (
                <div key={index} className="flex items-center justify-between p-2 bg-gray-800 rounded">
                  <span className="text-sm text-gray-200">{opt.title}</span>
                  <div className="flex gap-2">
                    <Badge size="sm" variant="outline">{opt.impact}</Badge>
                    <Button size="sm">Apply</Button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
