import React, { useState, useEffect, useCallback } from 'react'
import { Shield, Plus, Trash2, TestTube } from 'lucide-react'
import apiService from '@/services/api'

interface ProtectedPathsEditorProps {
  projectId: number
}

const PRESET_PATTERNS = ['.env*', 'migrations/**', '*.key', '*.pem', '*.secret', 'credentials.*']

export const ProtectedPathsEditor: React.FC<ProtectedPathsEditorProps> = ({ projectId }) => {
  const [patterns, setPatterns] = useState<string[]>([])
  const [newPattern, setNewPattern] = useState('')
  const [testPath, setTestPath] = useState('')
  const [testResult, setTestResult] = useState<{ matched: boolean; pattern?: string } | null>(null)
  const [saving, setSaving] = useState(false)

  const loadPatterns = useCallback(async () => {
    try {
      const res = await apiService.get(`/projects/${projectId}/protected-paths`)
      setPatterns(res.data?.patterns || [])
    } catch (err) {
      console.error('Failed to load protected paths:', err)
    }
  }, [projectId])

  useEffect(() => { loadPatterns() }, [loadPatterns])

  const savePatterns = async (updatedPatterns: string[]) => {
    setSaving(true)
    try {
      await apiService.put(`/projects/${projectId}/protected-paths`, { patterns: updatedPatterns })
      setPatterns(updatedPatterns)
    } catch (err) {
      console.error('Failed to save:', err)
    } finally {
      setSaving(false)
    }
  }

  const addPattern = () => {
    const p = newPattern.trim()
    if (!p || patterns.includes(p)) return
    savePatterns([...patterns, p])
    setNewPattern('')
  }

  const removePattern = (idx: number) => {
    savePatterns(patterns.filter((_, i) => i !== idx))
  }

  const addPreset = (preset: string) => {
    if (patterns.includes(preset)) return
    savePatterns([...patterns, preset])
  }

  const testPathMatch = () => {
    if (!testPath.trim()) return
    for (const pattern of patterns) {
      const regex = patternToRegex(pattern)
      if (regex.test(testPath.trim())) {
        setTestResult({ matched: true, pattern })
        return
      }
    }
    setTestResult({ matched: false })
  }

  return (
    <div className="space-y-4">
      {/* Current Patterns */}
      <div className="space-y-2">
        {patterns.map((p, i) => (
          <div key={i} className="flex items-center justify-between bg-black/40 border border-gray-700 rounded-lg px-3 py-2">
            <span className="text-sm text-gray-300 font-mono">{p}</span>
            <button onClick={() => removePattern(i)} className="text-gray-500 hover:text-red-400">
              <Trash2 size={14} />
            </button>
          </div>
        ))}
      </div>

      {/* Add New */}
      <div className="flex gap-2">
        <input
          value={newPattern}
          onChange={e => setNewPattern(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && addPattern()}
          placeholder="*.env or migrations/**"
          className="flex-1 bg-black/50 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white"
        />
        <button
          onClick={addPattern}
          disabled={!newPattern.trim() || saving}
          className="flex items-center gap-1 px-3 py-2 bg-red-600 hover:bg-red-500 disabled:opacity-50 rounded-lg text-white text-sm"
        >
          <Plus size={14} />
          Add
        </button>
      </div>

      {/* Presets */}
      <div>
        <p className="text-xs text-gray-500 mb-2">Quick Add Presets:</p>
        <div className="flex flex-wrap gap-1.5">
          {PRESET_PATTERNS.filter(p => !patterns.includes(p)).map(preset => (
            <button
              key={preset}
              onClick={() => addPreset(preset)}
              className="px-2 py-1 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded text-xs text-gray-400 hover:text-white transition-colors"
            >
              + {preset}
            </button>
          ))}
        </div>
      </div>

      {/* Test Path */}
      <div className="border-t border-gray-800 pt-4">
        <p className="text-xs text-gray-400 mb-2 flex items-center gap-1">
          <TestTube size={12} />
          Test a path against your patterns
        </p>
        <div className="flex gap-2">
          <input
            value={testPath}
            onChange={e => { setTestPath(e.target.value); setTestResult(null) }}
            placeholder="src/.env.local"
            className="flex-1 bg-black/50 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white"
          />
          <button
            onClick={testPathMatch}
            disabled={!testPath.trim()}
            className="px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm text-white disabled:opacity-50"
          >
            Test
          </button>
        </div>
        {testResult && (
          <p className={`text-xs mt-2 ${testResult.matched ? 'text-red-400' : 'text-green-400'}`}>
            {testResult.matched
              ? `BLOCKED - matches pattern "${testResult.pattern}"`
              : 'ALLOWED - no pattern matches'}
          </p>
        )}
      </div>
    </div>
  )
}

function patternToRegex(pattern: string): RegExp {
  let regexStr = pattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*\*/g, '{{GLOBSTAR}}')
    .replace(/\*/g, '[^/]*')
    .replace(/\?/g, '[^/]')
    .replace(/\{\{GLOBSTAR\}\}/g, '.*')
  return new RegExp(`(^|/)${regexStr}$`)
}

export default ProtectedPathsEditor
