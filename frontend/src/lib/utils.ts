// APEX.BUILD Utility Functions
// Cyberpunk development platform utilities

import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

// Merge Tailwind classes with proper precedence
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Format file sizes in human-readable format
export function formatFileSize(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let unitIndex = 0

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }

  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

// Format relative time (e.g., "2 minutes ago")
export function formatRelativeTime(date: string | Date): string {
  const now = new Date()
  const target = new Date(date)
  const diffMs = now.getTime() - target.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 30) return `${diffDays}d ago`

  return target.toLocaleDateString()
}

// Format AI cost in USD
export function formatCost(cost: number): string {
  if (cost === 0) return 'Free'
  if (cost < 0.001) return '<$0.001'
  return `$${cost.toFixed(3)}`
}

// Generate random ID
export function generateId(): string {
  return Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15)
}

// Copy text to clipboard
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

// Get file icon based on extension
export function getFileIcon(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || ''
  const iconMap: Record<string, string> = {
    'js': '📜', 'ts': '🔷', 'jsx': '⚛️', 'tsx': '🔹',
    'py': '🐍', 'java': '☕', 'go': '🐹', 'rs': '🦀',
    'html': '🌐', 'css': '🎨', 'json': '📋', 'md': '📖'
  }
  return iconMap[ext] || '📄'
}