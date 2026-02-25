// APEX.BUILD 22nd Century Steampunk Theme
// Fusion of cyberpunk neon + Victorian brass + industrial machinery aesthetics
// Designed to make every other IDE look like a plain text editor

import { Theme } from '@/types'

export const steampunkTheme: Theme = {
  id: 'steampunk',
  name: '22nd Century Steampunk',
  colors: {
    primary: '#d4a574',       // Tarnished Brass
    secondary: '#8b4513',     // Saddle Brown
    accent: '#ff6b35',        // Forge Orange
    background: '#0a0806',    // Soot Black
    surface: '#1a1510',       // Dark Mahogany
    text: '#e8d5b7',          // Parchment
    textSecondary: '#b8956a', // Aged Copper
    error: '#c0392b',         // Furnace Red
    warning: '#d4a574',       // Brass Warning
    success: '#27ae60',       // Verdigris Green
    info: '#2980b9',          // Blueprint Blue
  },
  effects: {
    glassMorphism: `
      background: rgba(26, 21, 16, 0.85);
      backdrop-filter: blur(12px) saturate(180%);
      border: 1px solid rgba(212, 165, 116, 0.25);
      box-shadow:
        0 8px 32px rgba(0, 0, 0, 0.4),
        inset 0 1px 0 rgba(212, 165, 116, 0.1);
    `,
    neonGlow: `
      box-shadow:
        0 0 5px rgba(212, 165, 116, 0.4),
        0 0 15px rgba(212, 165, 116, 0.2),
        0 0 30px rgba(255, 107, 53, 0.1);
    `,
    holographic: `
      background: linear-gradient(45deg, #d4a574, #ff6b35, #8b4513, #d4a574);
      background-size: 300% 300%;
      animation: steampunkShift 4s ease infinite;
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    `,
  },
}

// Steampunk-specific CSS classes for Tailwind
export const steampunkClasses = {
  // Brass plate effect
  brassPlate: [
    'bg-gradient-to-br from-amber-700/20 via-yellow-600/10 to-amber-800/20',
    'border border-amber-600/30',
    'shadow-lg shadow-amber-900/20',
  ].join(' '),

  // Riveted panel
  rivetedPanel: [
    'relative',
    'bg-gradient-to-b from-stone-900/80 to-stone-950/90',
    'border-2 border-amber-700/40',
    'rounded-lg',
    'shadow-[inset_0_2px_4px_rgba(0,0,0,0.3)]',
  ].join(' '),

  // Gear spinner
  gearSpin: 'animate-[spin_8s_linear_infinite]',

  // Steam vent glow
  steamVent: [
    'bg-gradient-to-t from-amber-500/20 to-transparent',
    'blur-sm',
  ].join(' '),

  // Gauge meter
  gaugeMeter: [
    'rounded-full',
    'bg-gradient-to-r from-red-600 via-amber-500 to-green-500',
    'shadow-inner',
  ].join(' '),

  // Victorian border
  victorianBorder: [
    'border-2 border-amber-600/50',
    'rounded-lg',
    'shadow-[0_0_15px_rgba(212,165,116,0.15)]',
    'relative',
    'before:absolute before:inset-0 before:rounded-lg',
    'before:border before:border-amber-500/10',
    'before:m-1',
  ].join(' '),

  // Parchment text
  parchmentText: 'text-amber-200/90 font-serif tracking-wide',

  // Forge glow animation
  forgeGlow: 'animate-[forgeGlow_3s_ease-in-out_infinite]',

  // Scanline overlay
  scanlineOverlay: 'bg-[repeating-linear-gradient(0deg,transparent,transparent_2px,rgba(0,0,0,0.03)_2px,rgba(0,0,0,0.03)_4px)]',
}

// Steampunk Monaco Editor theme
export const steampunkMonacoTheme = {
  base: 'vs-dark' as const,
  inherit: true,
  rules: [
    { token: 'comment', foreground: '8B7355', fontStyle: 'italic' },
    { token: 'keyword', foreground: 'D4A574', fontStyle: 'bold' },
    { token: 'string', foreground: 'FF6B35' },
    { token: 'number', foreground: 'B8956A' },
    { token: 'function', foreground: 'D4A574' },
    { token: 'variable', foreground: 'E8D5B7' },
    { token: 'type', foreground: '8B4513' },
    { token: 'constant', foreground: 'FF6B35' },
    { token: 'operator', foreground: 'B8956A' },
    { token: 'regexp', foreground: 'C0392B' },
    { token: 'attribute.name', foreground: 'D4A574' },
    { token: 'attribute.value', foreground: '27AE60' },
    { token: 'tag', foreground: 'FF6B35' },
    { token: 'delimiter', foreground: '8B7355' },
  ],
  colors: {
    'editor.background': '#0A0806',
    'editor.foreground': '#E8D5B7',
    'editor.lineHighlightBackground': '#1A151033',
    'editor.selectionBackground': '#D4A57433',
    'editor.inactiveSelectionBackground': '#D4A5741A',
    'editorCursor.foreground': '#FF6B35',
    'editorLineNumber.foreground': '#8B735566',
    'editorLineNumber.activeForeground': '#D4A574',
    'editor.selectionHighlightBackground': '#FF6B3533',
    'editorGutter.background': '#0D0A07',
    'editorWidget.background': '#1A1510',
    'editorWidget.border': '#D4A57440',
    'minimap.background': '#0A0806',
  },
}

// Steampunk-specific keyframe animations
export const steampunkAnimations = `
  @keyframes steampunkShift {
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes forgeGlow {
    0%, 100% {
      box-shadow: 0 0 5px rgba(255, 107, 53, 0.3), 0 0 15px rgba(212, 165, 116, 0.15);
    }
    50% {
      box-shadow: 0 0 15px rgba(255, 107, 53, 0.5), 0 0 30px rgba(212, 165, 116, 0.25);
    }
  }

  @keyframes gearRotate {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  @keyframes gearRotateReverse {
    from { transform: rotate(360deg); }
    to { transform: rotate(0deg); }
  }

  @keyframes steamRise {
    0% { transform: translateY(0) scale(1); opacity: 0.4; }
    50% { transform: translateY(-30px) scale(1.3); opacity: 0.2; }
    100% { transform: translateY(-60px) scale(1.6); opacity: 0; }
  }

  @keyframes pistonPump {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-8px); }
  }

  @keyframes brassShimmer {
    0% { background-position: -200% center; }
    100% { background-position: 200% center; }
  }

  @keyframes pressureGauge {
    0%, 100% { transform: rotate(-45deg); }
    25% { transform: rotate(-30deg); }
    50% { transform: rotate(-15deg); }
    75% { transform: rotate(-25deg); }
  }

  @keyframes voltageArc {
    0%, 100% { opacity: 0; }
    5% { opacity: 1; }
    10% { opacity: 0; }
    15% { opacity: 0.8; }
    20% { opacity: 0; }
  }

  @keyframes tickerTape {
    from { transform: translateX(0); }
    to { transform: translateX(-50%); }
  }

  .animate-gear { animation: gearRotate 8s linear infinite; }
  .animate-gear-reverse { animation: gearRotateReverse 6s linear infinite; }
  .animate-steam { animation: steamRise 3s ease-out infinite; }
  .animate-piston { animation: pistonPump 2s ease-in-out infinite; }
  .animate-brass-shimmer {
    background: linear-gradient(90deg, transparent, rgba(212,165,116,0.3), transparent);
    background-size: 200% 100%;
    animation: brassShimmer 3s ease infinite;
  }
  .animate-voltage { animation: voltageArc 4s linear infinite; }
  .animate-ticker { animation: tickerTape 20s linear infinite; }
`

export default steampunkTheme
