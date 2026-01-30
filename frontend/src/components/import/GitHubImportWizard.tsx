// GitHubImportWizard.tsx
// One-click GitHub repository import wizard for APEX.BUILD
// Similar to Replit's replit.new/URL feature

import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import apiService from '@/services/api';

// Types
interface DetectedStack {
  language: string;
  framework: string;
  package_manager: string;
  entry_point: string;
}

interface RepoValidation {
  valid: boolean;
  error?: string;
  hint?: string;
  private?: boolean;
  owner?: string;
  repo?: string;
  name?: string;
  description?: string;
  default_branch?: string;
  language?: string;
  size?: number;
  stars?: number;
  forks?: number;
  detected_stack?: DetectedStack;
}

interface ImportResponse {
  project_id: number;
  project_name: string;
  language: string;
  framework: string;
  detected_stack: DetectedStack;
  file_count: number;
  status: string;
  message: string;
  import_duration_ms: number;
  repository_url: string;
  default_branch: string;
}

type ImportStep = 'url' | 'validating' | 'configure' | 'importing' | 'success' | 'error';

// Language icons/colors mapping
const languageConfig: Record<string, { icon: string; color: string; bg: string }> = {
  javascript: { icon: 'JS', color: '#f7df1e', bg: 'rgba(247, 223, 30, 0.2)' },
  typescript: { icon: 'TS', color: '#3178c6', bg: 'rgba(49, 120, 198, 0.2)' },
  python: { icon: 'PY', color: '#3776ab', bg: 'rgba(55, 118, 171, 0.2)' },
  go: { icon: 'GO', color: '#00add8', bg: 'rgba(0, 173, 216, 0.2)' },
  rust: { icon: 'RS', color: '#dea584', bg: 'rgba(222, 165, 132, 0.2)' },
  java: { icon: 'JV', color: '#007396', bg: 'rgba(0, 115, 150, 0.2)' },
  ruby: { icon: 'RB', color: '#cc342d', bg: 'rgba(204, 52, 45, 0.2)' },
  php: { icon: 'PHP', color: '#777bb4', bg: 'rgba(119, 123, 180, 0.2)' },
  dart: { icon: 'DT', color: '#0175c2', bg: 'rgba(1, 117, 194, 0.2)' },
  cpp: { icon: 'C++', color: '#00599c', bg: 'rgba(0, 89, 156, 0.2)' },
  csharp: { icon: 'C#', color: '#239120', bg: 'rgba(35, 145, 32, 0.2)' },
  swift: { icon: 'SW', color: '#f05138', bg: 'rgba(240, 81, 56, 0.2)' },
  kotlin: { icon: 'KT', color: '#7f52ff', bg: 'rgba(127, 82, 255, 0.2)' },
};

// Framework badges
const frameworkConfig: Record<string, { name: string; color: string }> = {
  react: { name: 'React', color: '#61dafb' },
  nextjs: { name: 'Next.js', color: '#000000' },
  vue: { name: 'Vue.js', color: '#4fc08d' },
  nuxt: { name: 'Nuxt', color: '#00dc82' },
  angular: { name: 'Angular', color: '#dd0031' },
  svelte: { name: 'Svelte', color: '#ff3e00' },
  express: { name: 'Express', color: '#000000' },
  fastapi: { name: 'FastAPI', color: '#009688' },
  django: { name: 'Django', color: '#092e20' },
  flask: { name: 'Flask', color: '#000000' },
  rails: { name: 'Rails', color: '#cc0000' },
  laravel: { name: 'Laravel', color: '#ff2d20' },
  spring: { name: 'Spring', color: '#6db33f' },
  gin: { name: 'Gin', color: '#00add8' },
  actix: { name: 'Actix', color: '#dea584' },
  flutter: { name: 'Flutter', color: '#02569b' },
  vite: { name: 'Vite', color: '#646cff' },
};

interface GitHubImportWizardProps {
  onClose?: () => void;
}

export const GitHubImportWizard: React.FC<GitHubImportWizardProps> = ({ onClose }) => {
  const navigate = useNavigate();

  // State
  const [step, setStep] = useState<ImportStep>('url');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [showToken, setShowToken] = useState(false);
  const [validation, setValidation] = useState<RepoValidation | null>(null);
  const [projectName, setProjectName] = useState('');
  const [description, setDescription] = useState('');
  const [isPublic, setIsPublic] = useState(false);
  const [importResult, setImportResult] = useState<ImportResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);

  // URL validation with debounce
  const validateUrl = useCallback(async (inputUrl: string) => {
    if (!inputUrl.trim()) {
      setValidation(null);
      return;
    }

    // Quick client-side validation
    const urlPattern = /^(https?:\/\/)?(github\.com\/)?[\w-]+\/[\w.-]+\/?$/;
    if (!urlPattern.test(inputUrl.replace('.git', ''))) {
      setValidation({
        valid: false,
        error: 'Invalid GitHub URL format',
        hint: 'Expected format: github.com/owner/repo or https://github.com/owner/repo',
      });
      return;
    }

    setStep('validating');

    try {
      const response = await fetch('/api/v1/projects/import/github/validate', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`,
        },
        body: JSON.stringify({ url: inputUrl, token }),
      });

      const data = await response.json();
      setValidation(data);

      if (data.valid) {
        setProjectName(data.name || '');
        setDescription(data.description || '');
        setStep('configure');
      } else {
        setStep('url');
      }
    } catch (err) {
      setValidation({
        valid: false,
        error: 'Failed to validate repository',
      });
      setStep('url');
    }
  }, [token]);

  // Handle URL input change with debounce
  useEffect(() => {
    const timer = setTimeout(() => {
      if (url.trim() && step === 'url') {
        // Don't auto-validate, wait for button click
      }
    }, 500);

    return () => clearTimeout(timer);
  }, [url, step]);

  // Handle import
  const handleImport = async () => {
    if (!validation?.valid) return;

    setStep('importing');
    setProgress(10);

    // Simulate progress updates
    const progressInterval = setInterval(() => {
      setProgress((prev) => {
        if (prev >= 90) {
          clearInterval(progressInterval);
          return 90;
        }
        return prev + Math.random() * 15;
      });
    }, 500);

    try {
      const response = await fetch('/api/v1/projects/import/github', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('apex_access_token')}`,
        },
        body: JSON.stringify({
          url,
          project_name: projectName,
          description,
          is_public: isPublic,
          token,
        }),
      });

      clearInterval(progressInterval);

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Import failed');
      }

      const data: ImportResponse = await response.json();
      setImportResult(data);
      setProgress(100);
      setStep('success');
    } catch (err) {
      clearInterval(progressInterval);
      setError(err instanceof Error ? err.message : 'Import failed');
      setStep('error');
    }
  };

  // Reset wizard
  const handleReset = () => {
    setStep('url');
    setUrl('');
    setToken('');
    setValidation(null);
    setProjectName('');
    setDescription('');
    setIsPublic(false);
    setImportResult(null);
    setError(null);
    setProgress(0);
  };

  // Navigate to project
  const handleOpenProject = () => {
    if (importResult) {
      navigate(`/ide/${importResult.project_id}`);
    }
  };

  // Render language badge
  const renderLanguageBadge = (language: string) => {
    const config = languageConfig[language.toLowerCase()] || {
      icon: language.substring(0, 2).toUpperCase(),
      color: '#6b7280',
      bg: 'rgba(107, 114, 128, 0.2)',
    };

    return (
      <span
        className="inline-flex items-center px-2 py-1 rounded text-xs font-medium"
        style={{ backgroundColor: config.bg, color: config.color }}
      >
        {config.icon}
      </span>
    );
  };

  // Render framework badge
  const renderFrameworkBadge = (framework: string) => {
    const config = frameworkConfig[framework.toLowerCase()];
    if (!config) return null;

    return (
      <span
        className="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-gray-800 text-gray-300"
        style={{ borderColor: config.color, borderWidth: 1 }}
      >
        {config.name}
      </span>
    );
  };

  return (
    <div className="w-full max-w-2xl bg-gray-900 rounded-2xl border border-gray-700 shadow-2xl overflow-hidden">
      {/* Header with close button */}
      <div className="flex items-center justify-between p-6 border-b border-gray-700">
        <div>
          <h1 className="text-2xl font-bold text-white">
            Import from GitHub
          </h1>
          <p className="text-gray-400 text-sm mt-1">
            Clone any public or private repository and start coding in seconds
          </p>
        </div>
        {onClose && (
          <button
            onClick={onClose}
            className="p-2 text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        )}
      </div>

      <div className="p-6">
          {/* URL Input Step */}
          {(step === 'url' || step === 'validating') && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  GitHub Repository URL
                </label>
                <div className="relative">
                  <input
                    type="text"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    placeholder="github.com/owner/repo or paste full URL"
                    className="w-full px-4 py-3 bg-gray-900 border border-gray-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:border-transparent"
                    disabled={step === 'validating'}
                  />
                  {step === 'validating' && (
                    <div className="absolute right-3 top-1/2 -translate-y-1/2">
                      <div className="w-5 h-5 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin" />
                    </div>
                  )}
                </div>
              </div>

              {/* Token input for private repos */}
              <div>
                <button
                  type="button"
                  onClick={() => setShowToken(!showToken)}
                  className="text-sm text-cyan-400 hover:text-cyan-300 flex items-center gap-1"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                  </svg>
                  {showToken ? 'Hide' : 'Add'} Personal Access Token (for private repos)
                </button>

                {showToken && (
                  <input
                    type="password"
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    placeholder="ghp_xxxxxxxxxxxx"
                    className="mt-2 w-full px-4 py-2 bg-gray-900 border border-gray-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:border-transparent text-sm"
                  />
                )}
              </div>

              {/* Validation error */}
              {validation && !validation.valid && (
                <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-3">
                  <p className="text-red-400 text-sm">{validation.error}</p>
                  {validation.hint && (
                    <p className="text-gray-400 text-xs mt-1">{validation.hint}</p>
                  )}
                </div>
              )}

              {/* Validate button */}
              <button
                onClick={() => validateUrl(url)}
                disabled={!url.trim() || step === 'validating'}
                className="w-full py-3 px-4 bg-gradient-to-r from-cyan-500 to-blue-500 text-white font-medium rounded-lg hover:from-cyan-600 hover:to-blue-600 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 focus:ring-offset-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
              >
                {step === 'validating' ? 'Validating...' : 'Continue'}
              </button>
            </div>
          )}

          {/* Configure Step */}
          {step === 'configure' && validation?.valid && (
            <div className="space-y-6">
              {/* Repository preview */}
              <div className="bg-gray-900/50 rounded-lg p-4 border border-gray-700">
                <div className="flex items-start gap-4">
                  <div className="w-12 h-12 bg-gray-800 rounded-lg flex items-center justify-center">
                    <svg className="w-6 h-6 text-gray-400" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                    </svg>
                  </div>
                  <div className="flex-1 min-w-0">
                    <h3 className="text-white font-medium truncate">
                      {validation.owner}/{validation.repo}
                    </h3>
                    {validation.description && (
                      <p className="text-gray-400 text-sm mt-1 line-clamp-2">
                        {validation.description}
                      </p>
                    )}
                    <div className="flex items-center gap-3 mt-2">
                      {validation.detected_stack?.language && renderLanguageBadge(validation.detected_stack.language)}
                      {validation.detected_stack?.framework && renderFrameworkBadge(validation.detected_stack.framework)}
                      {validation.stars !== undefined && validation.stars > 0 && (
                        <span className="text-gray-400 text-xs flex items-center gap-1">
                          <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M12 .587l3.668 7.568 8.332 1.151-6.064 5.828 1.48 8.279-7.416-3.967-7.417 3.967 1.481-8.279-6.064-5.828 8.332-1.151z"/>
                          </svg>
                          {validation.stars.toLocaleString()}
                        </span>
                      )}
                      {validation.private && (
                        <span className="text-yellow-400 text-xs flex items-center gap-1">
                          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                          </svg>
                          Private
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              </div>

              {/* Project name */}
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Project Name
                </label>
                <input
                  type="text"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  className="w-full px-4 py-2 bg-gray-900 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:border-transparent"
                />
              </div>

              {/* Description */}
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Description
                </label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={2}
                  className="w-full px-4 py-2 bg-gray-900 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:border-transparent resize-none"
                />
              </div>

              {/* Visibility toggle */}
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-sm font-medium text-gray-300">Make project public</span>
                  <p className="text-xs text-gray-500">Anyone can view public projects</p>
                </div>
                <button
                  onClick={() => setIsPublic(!isPublic)}
                  className={`relative w-12 h-6 rounded-full transition-colors ${
                    isPublic ? 'bg-cyan-500' : 'bg-gray-600'
                  }`}
                >
                  <span
                    className={`absolute top-1 w-4 h-4 bg-white rounded-full transition-transform ${
                      isPublic ? 'translate-x-7' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {/* Detected stack info */}
              {validation.detected_stack && (
                <div className="bg-gray-900/50 rounded-lg p-4 border border-gray-700">
                  <h4 className="text-sm font-medium text-gray-300 mb-2">Detected Stack</h4>
                  <div className="grid grid-cols-2 gap-2 text-sm">
                    <div>
                      <span className="text-gray-500">Language:</span>
                      <span className="text-white ml-2">{validation.detected_stack.language}</span>
                    </div>
                    {validation.detected_stack.framework && (
                      <div>
                        <span className="text-gray-500">Framework:</span>
                        <span className="text-white ml-2">{validation.detected_stack.framework}</span>
                      </div>
                    )}
                    {validation.detected_stack.package_manager && (
                      <div>
                        <span className="text-gray-500">Package Manager:</span>
                        <span className="text-white ml-2">{validation.detected_stack.package_manager}</span>
                      </div>
                    )}
                    {validation.detected_stack.entry_point && (
                      <div>
                        <span className="text-gray-500">Entry Point:</span>
                        <span className="text-white ml-2">{validation.detected_stack.entry_point}</span>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Action buttons */}
              <div className="flex gap-3">
                <button
                  onClick={() => setStep('url')}
                  className="px-4 py-2 text-gray-400 hover:text-white transition-colors"
                >
                  Back
                </button>
                <button
                  onClick={handleImport}
                  disabled={!projectName.trim()}
                  className="flex-1 py-3 px-4 bg-gradient-to-r from-cyan-500 to-blue-500 text-white font-medium rounded-lg hover:from-cyan-600 hover:to-blue-600 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 focus:ring-offset-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                >
                  Import Repository
                </button>
              </div>
            </div>
          )}

          {/* Importing Step */}
          {step === 'importing' && (
            <div className="text-center py-8">
              <div className="w-16 h-16 mx-auto mb-4 relative">
                <svg className="w-16 h-16 animate-spin text-cyan-500" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
              </div>
              <h3 className="text-xl font-medium text-white mb-2">Importing Repository</h3>
              <p className="text-gray-400 mb-4">Cloning files and setting up your project...</p>

              {/* Progress bar */}
              <div className="w-full bg-gray-700 rounded-full h-2 mb-2">
                <div
                  className="bg-gradient-to-r from-cyan-500 to-blue-500 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>
              <p className="text-sm text-gray-500">{Math.round(progress)}% complete</p>
            </div>
          )}

          {/* Success Step */}
          {step === 'success' && importResult && (
            <div className="text-center py-8">
              <div className="w-16 h-16 mx-auto mb-4 bg-green-500/20 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              </div>
              <h3 className="text-xl font-medium text-white mb-2">Import Complete!</h3>
              <p className="text-gray-400 mb-6">
                Imported {importResult.file_count} files in {(importResult.import_duration_ms / 1000).toFixed(1)}s
              </p>

              {/* Project summary */}
              <div className="bg-gray-900/50 rounded-lg p-4 border border-gray-700 text-left mb-6">
                <div className="flex items-center gap-3 mb-3">
                  <span className="text-white font-medium">{importResult.project_name}</span>
                  {renderLanguageBadge(importResult.language)}
                  {importResult.framework && renderFrameworkBadge(importResult.framework)}
                </div>
                <div className="text-sm text-gray-400">
                  <p>Branch: {importResult.default_branch}</p>
                  <p>Source: {importResult.repository_url}</p>
                </div>
              </div>

              <div className="flex gap-3">
                <button
                  onClick={handleReset}
                  className="px-4 py-2 text-gray-400 hover:text-white transition-colors"
                >
                  Import Another
                </button>
                <button
                  onClick={handleOpenProject}
                  className="flex-1 py-3 px-4 bg-gradient-to-r from-cyan-500 to-blue-500 text-white font-medium rounded-lg hover:from-cyan-600 hover:to-blue-600 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 focus:ring-offset-gray-800 transition-all"
                >
                  Open in IDE
                </button>
              </div>
            </div>
          )}

          {/* Error Step */}
          {step === 'error' && (
            <div className="text-center py-8">
              <div className="w-16 h-16 mx-auto mb-4 bg-red-500/20 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </div>
              <h3 className="text-xl font-medium text-white mb-2">Import Failed</h3>
              <p className="text-red-400 mb-6">{error}</p>

              <button
                onClick={handleReset}
                className="w-full py-3 px-4 bg-gradient-to-r from-cyan-500 to-blue-500 text-white font-medium rounded-lg hover:from-cyan-600 hover:to-blue-600 focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 focus:ring-offset-gray-800 transition-all"
              >
                Try Again
              </button>
            </div>
          )}

          {/* Footer help text */}
          <div className="mt-6 text-center text-sm text-gray-500">
            <p>
              Tip: You can also paste URLs directly like{' '}
              <code className="bg-gray-700 px-2 py-0.5 rounded text-cyan-400">
                apex.build/import/github.com/owner/repo
              </code>
            </p>
          </div>
        </div>
      </div>
  );
};

export default GitHubImportWizard;
