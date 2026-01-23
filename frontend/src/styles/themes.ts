// APEX.BUILD Cyberpunk Theme System
// Beautiful, futuristic themes that make Replit look ancient

import { Theme } from '@/types'

export const themes: Record<string, Theme> = {
  cyberpunk: {
    id: 'cyberpunk',
    name: 'Neon Cyberpunk',
    colors: {
      primary: '#00f5ff',      // Electric Cyan
      secondary: '#ff0080',    // Hot Pink
      accent: '#39ff14',       // Acid Green
      background: '#0a0a0a',   // Deep Space
      surface: '#1a1a2e',      // Dark Steel
      text: '#ffffff',         // Neon White
      textSecondary: '#e0e0e0',// Silver
      error: '#ff4444',        // Cyber Red
      warning: '#ffaa00',      // Electric Orange
      success: '#39ff14',      // Acid Green
      info: '#00f5ff',         // Electric Cyan
    },
    effects: {
      glassMorphism: `
        background: rgba(26, 26, 46, 0.8);
        backdrop-filter: blur(10px);
        border: 1px solid rgba(0, 245, 255, 0.2);
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
      `,
      neonGlow: `
        box-shadow:
          0 0 5px currentColor,
          0 0 10px currentColor,
          0 0 20px currentColor;
      `,
      holographic: `
        background: linear-gradient(45deg, #00f5ff, #ff0080, #39ff14, #8a2be2);
        background-size: 300% 300%;
        animation: holographicShift 3s ease infinite;
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      `,
    },
  },

  matrix: {
    id: 'matrix',
    name: 'Matrix Digital',
    colors: {
      primary: '#00ff41',      // Matrix Green
      secondary: '#ff4500',    // Digital Orange
      accent: '#00ffff',       // Electric Cyan
      background: '#000000',   // Pure Black
      surface: '#001100',      // Dark Green
      text: '#00ff41',         // Matrix Green
      textSecondary: '#90ee90',// Light Green
      error: '#ff0000',        // Pure Red
      warning: '#ffa500',      // Orange
      success: '#00ff41',      // Matrix Green
      info: '#00ffff',         // Cyan
    },
    effects: {
      glassMorphism: `
        background: rgba(0, 17, 0, 0.8);
        backdrop-filter: blur(10px);
        border: 1px solid rgba(0, 255, 65, 0.2);
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
      `,
      neonGlow: `
        box-shadow:
          0 0 5px #00ff41,
          0 0 10px #00ff41,
          0 0 20px #00ff41;
      `,
      holographic: `
        background: linear-gradient(45deg, #00ff41, #00ffff, #90ee90);
        background-size: 300% 300%;
        animation: matrixShift 2s ease infinite;
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      `,
    },
  },

  synthwave: {
    id: 'synthwave',
    name: 'Synthwave Retro',
    colors: {
      primary: '#ff006e',      // Hot Pink
      secondary: '#8338ec',    // Electric Purple
      accent: '#ffbe0b',       // Golden Yellow
      background: '#0f0f23',   // Deep Purple
      surface: '#1a1a2e',      // Dark Navy
      text: '#ffffff',         // White
      textSecondary: '#dda0dd',// Plum
      error: '#ff073a',        // Neon Red
      warning: '#ffbe0b',      // Golden Yellow
      success: '#06ffa5',      // Neon Mint
      info: '#4361ee',         // Electric Blue
    },
    effects: {
      glassMorphism: `
        background: rgba(26, 26, 46, 0.8);
        backdrop-filter: blur(10px);
        border: 1px solid rgba(255, 0, 110, 0.2);
        box-shadow: 0 8px 32px rgba(131, 56, 236, 0.3);
      `,
      neonGlow: `
        box-shadow:
          0 0 5px #ff006e,
          0 0 10px #ff006e,
          0 0 20px #8338ec;
      `,
      holographic: `
        background: linear-gradient(45deg, #ff006e, #8338ec, #ffbe0b, #06ffa5);
        background-size: 400% 400%;
        animation: synthwaveShift 4s ease infinite;
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      `,
    },
  },

  neonCity: {
    id: 'neonCity',
    name: 'Neon City',
    colors: {
      primary: '#00d4ff',      // Sky Blue
      secondary: '#ff0080',    // Hot Pink
      accent: '#39ff14',       // Lime Green
      background: '#0d1421',   // Dark Blue
      surface: '#1e2a3a',      // Steel Blue
      text: '#ffffff',         // White
      textSecondary: '#b0c4de', // Light Steel Blue
      error: '#ff4757',        // Red
      warning: '#ffa502',      // Orange
      success: '#2ed573',      // Green
      info: '#3742fa',         // Blue
    },
    effects: {
      glassMorphism: `
        background: rgba(30, 42, 58, 0.8);
        backdrop-filter: blur(10px);
        border: 1px solid rgba(0, 212, 255, 0.2);
        box-shadow: 0 8px 32px rgba(13, 20, 33, 0.4);
      `,
      neonGlow: `
        box-shadow:
          0 0 5px #00d4ff,
          0 0 10px #00d4ff,
          0 0 20px #ff0080;
      `,
      holographic: `
        background: linear-gradient(45deg, #00d4ff, #ff0080, #39ff14, #3742fa);
        background-size: 350% 350%;
        animation: neonCityShift 3.5s ease infinite;
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      `,
    },
  },
}

// CSS animations for themes
export const globalAnimations = `
  @keyframes holographicShift {
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes matrixShift {
    0% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes synthwaveShift {
    0% { background-position: 0% 50%; }
    25% { background-position: 100% 0%; }
    50% { background-position: 100% 100%; }
    75% { background-position: 0% 100%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes neonCityShift {
    0% { background-position: 0% 50%; }
    33% { background-position: 100% 0%; }
    66% { background-position: 50% 100%; }
    100% { background-position: 0% 50%; }
  }

  @keyframes digitalRain {
    0% { transform: translateY(-100%); opacity: 0; }
    10% { opacity: 1; }
    90% { opacity: 1; }
    100% { transform: translateY(100vh); opacity: 0; }
  }

  @keyframes particleFloat {
    0%, 100% { transform: translateY(0px) rotate(0deg); opacity: 1; }
    50% { transform: translateY(-20px) rotate(180deg); opacity: 0.5; }
  }

  @keyframes glowPulse {
    0%, 100% {
      box-shadow: 0 0 5px currentColor;
      transform: scale(1);
    }
    50% {
      box-shadow: 0 0 20px currentColor, 0 0 30px currentColor;
      transform: scale(1.02);
    }
  }

  @keyframes typewriter {
    from { width: 0; }
    to { width: 100%; }
  }

  @keyframes blink {
    0%, 50% { border-color: transparent; }
    51%, 100% { border-color: currentColor; }
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateY(20px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes scaleIn {
    from {
      opacity: 0;
      transform: scale(0.8);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .animate-holographic {
    animation: holographicShift 3s ease infinite;
  }

  .animate-glow-pulse {
    animation: glowPulse 2s ease-in-out infinite;
  }

  .animate-slide-in {
    animation: slideIn 0.3s ease-out;
  }

  .animate-fade-in {
    animation: fadeIn 0.3s ease-out;
  }

  .animate-scale-in {
    animation: scaleIn 0.3s ease-out;
  }
`

// Monaco Editor themes for each variant
export const monacoThemes = {
  cyberpunk: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: '6A9955', fontStyle: 'italic' },
      { token: 'keyword', foreground: '00F5FF', fontStyle: 'bold' },
      { token: 'string', foreground: '39FF14' },
      { token: 'number', foreground: 'FF0080' },
      { token: 'function', foreground: '00F5FF' },
      { token: 'variable', foreground: 'FFFFFF' },
      { token: 'type', foreground: '8A2BE2' },
      { token: 'constant', foreground: 'FF0080' },
      { token: 'operator', foreground: '39FF14' },
    ],
    colors: {
      'editor.background': '#0A0A0A',
      'editor.foreground': '#FFFFFF',
      'editor.lineHighlightBackground': '#1A1A2E33',
      'editor.selectionBackground': '#00F5FF33',
      'editor.inactiveSelectionBackground': '#00F5FF1A',
      'editorCursor.foreground': '#00F5FF',
      'editorLineNumber.foreground': '#00F5FF66',
      'editorLineNumber.activeForeground': '#00F5FF',
      'editor.selectionHighlightBackground': '#FF008033',
    },
  },

  matrix: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: '90EE90', fontStyle: 'italic' },
      { token: 'keyword', foreground: '00FF41', fontStyle: 'bold' },
      { token: 'string', foreground: '00FFFF' },
      { token: 'number', foreground: 'FF4500' },
      { token: 'function', foreground: '00FF41' },
      { token: 'variable', foreground: '90EE90' },
      { token: 'type', foreground: '00FFFF' },
      { token: 'constant', foreground: 'FF4500' },
      { token: 'operator', foreground: '00FF41' },
    ],
    colors: {
      'editor.background': '#000000',
      'editor.foreground': '#00FF41',
      'editor.lineHighlightBackground': '#00110033',
      'editor.selectionBackground': '#00FF4133',
      'editor.inactiveSelectionBackground': '#00FF411A',
      'editorCursor.foreground': '#00FF41',
      'editorLineNumber.foreground': '#00FF4166',
      'editorLineNumber.activeForeground': '#00FF41',
      'editor.selectionHighlightBackground': '#00FFFF33',
    },
  },

  synthwave: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: 'DDA0DD', fontStyle: 'italic' },
      { token: 'keyword', foreground: 'FF006E', fontStyle: 'bold' },
      { token: 'string', foreground: 'FFBE0B' },
      { token: 'number', foreground: '8338EC' },
      { token: 'function', foreground: 'FF006E' },
      { token: 'variable', foreground: 'FFFFFF' },
      { token: 'type', foreground: '8338EC' },
      { token: 'constant', foreground: '06FFA5' },
      { token: 'operator', foreground: 'FFBE0B' },
    ],
    colors: {
      'editor.background': '#0F0F23',
      'editor.foreground': '#FFFFFF',
      'editor.lineHighlightBackground': '#1A1A2E33',
      'editor.selectionBackground': '#FF006E33',
      'editor.inactiveSelectionBackground': '#FF006E1A',
      'editorCursor.foreground': '#FF006E',
      'editorLineNumber.foreground': '#8338EC66',
      'editorLineNumber.activeForeground': '#FF006E',
      'editor.selectionHighlightBackground': '#8338EC33',
    },
  },

  neonCity: {
    base: 'vs-dark' as const,
    inherit: true,
    rules: [
      { token: 'comment', foreground: 'B0C4DE', fontStyle: 'italic' },
      { token: 'keyword', foreground: '00D4FF', fontStyle: 'bold' },
      { token: 'string', foreground: '39FF14' },
      { token: 'number', foreground: 'FF0080' },
      { token: 'function', foreground: '00D4FF' },
      { token: 'variable', foreground: 'FFFFFF' },
      { token: 'type', foreground: '3742FA' },
      { token: 'constant', foreground: 'FF0080' },
      { token: 'operator', foreground: '2ED573' },
    ],
    colors: {
      'editor.background': '#0D1421',
      'editor.foreground': '#FFFFFF',
      'editor.lineHighlightBackground': '#1E2A3A33',
      'editor.selectionBackground': '#00D4FF33',
      'editor.inactiveSelectionBackground': '#00D4FF1A',
      'editorCursor.foreground': '#00D4FF',
      'editorLineNumber.foreground': '#00D4FF66',
      'editorLineNumber.activeForeground': '#00D4FF',
      'editor.selectionHighlightBackground': '#FF008033',
    },
  },
}

// Language-specific configurations
export const languageConfigs = {
  typescript: {
    id: 'typescript',
    name: 'TypeScript',
    extensions: ['.ts', '.tsx'],
    icon: '⚡',
    color: '#3178C6',
    monacoLanguage: 'typescript',
    defaultCode: `// Welcome to APEX.BUILD TypeScript Environment
// 1000x faster than Replit!

interface User {
  id: number;
  name: string;
  isActive: boolean;
}

const createUser = (name: string): User => {
  return {
    id: Math.floor(Math.random() * 1000),
    name,
    isActive: true
  };
};

const user = createUser("APEX Developer");
console.log("User created:", user);
`,
    runCommand: 'tsx',
    buildCommand: 'tsc',
    testCommand: 'vitest',
  },

  javascript: {
    id: 'javascript',
    name: 'JavaScript',
    extensions: ['.js', '.jsx', '.mjs'],
    icon: '📜',
    color: '#F7DF1E',
    monacoLanguage: 'javascript',
    defaultCode: `// Welcome to APEX.BUILD JavaScript Environment
// Lightning fast execution!

const greetUser = (name) => {
  const greeting = \`Hello, \${name}! Welcome to APEX.BUILD!\`;
  return greeting;
};

const message = greetUser("Developer");
console.log(message);

// Multi-AI assistance available:
// - Claude for code review and debugging
// - GPT-4 for generation and refactoring
// - Gemini for explanations and completion
`,
    runCommand: 'node',
    testCommand: 'jest',
  },

  python: {
    id: 'python',
    name: 'Python',
    extensions: ['.py', '.pyw'],
    icon: '🐍',
    color: '#3776AB',
    monacoLanguage: 'python',
    defaultCode: `# Welcome to APEX.BUILD Python Environment
# Supercharged with AI assistance!

def fibonacci(n):
    """Generate fibonacci sequence up to n terms."""
    if n <= 0:
        return []
    elif n == 1:
        return [0]
    elif n == 2:
        return [0, 1]

    sequence = [0, 1]
    for i in range(2, n):
        sequence.append(sequence[i-1] + sequence[i-2])

    return sequence

# Generate and display fibonacci sequence
fib_sequence = fibonacci(10)
print(f"Fibonacci sequence: {fib_sequence}")

# APEX.BUILD features:
# - Multi-AI integration (Claude, GPT-4, Gemini)
# - Real-time collaboration
# - Beautiful cyberpunk interface
print("APEX.BUILD: Leaving Replit in the dust! 🚀")
`,
    runCommand: 'python3',
    testCommand: 'pytest',
  },

  go: {
    id: 'go',
    name: 'Go',
    extensions: ['.go'],
    icon: '🔷',
    color: '#00ADD8',
    monacoLanguage: 'go',
    defaultCode: `package main

import (
	"fmt"
	"time"
)

// User represents a user in APEX.BUILD
type User struct {
	ID       int       \`json:"id"\`
	Name     string    \`json:"name"\`
	JoinedAt time.Time \`json:"joined_at"\`
}

// NewUser creates a new user
func NewUser(name string) *User {
	return &User{
		ID:       42,
		Name:     name,
		JoinedAt: time.Now(),
	}
}

func main() {
	user := NewUser("Go Developer")
	fmt.Printf("Welcome %s to APEX.BUILD!\\n", user.Name)
	fmt.Printf("User ID: %d\\n", user.ID)
	fmt.Printf("Joined: %s\\n", user.JoinedAt.Format(time.RFC3339))

	fmt.Println("\\n🚀 APEX.BUILD Features:")
	fmt.Println("- Multi-AI assistance (Claude + GPT-4 + Gemini)")
	fmt.Println("- 1000x faster than Replit")
	fmt.Println("- Beautiful cyberpunk interface")
	fmt.Println("- Enterprise-grade performance")
}
`,
    runCommand: 'go run',
    buildCommand: 'go build',
    testCommand: 'go test',
  },

  rust: {
    id: 'rust',
    name: 'Rust',
    extensions: ['.rs'],
    icon: '🦀',
    color: '#CE422B',
    monacoLanguage: 'rust',
    defaultCode: `// Welcome to APEX.BUILD Rust Environment
// Memory safety meets AI power!

use std::collections::HashMap;

#[derive(Debug)]
struct Developer {
    name: String,
    experience: u32,
    languages: Vec<String>,
}

impl Developer {
    fn new(name: String, experience: u32) -> Self {
        Self {
            name,
            experience,
            languages: Vec::new(),
        }
    }

    fn add_language(&mut self, language: String) {
        self.languages.push(language);
    }

    fn introduce(&self) {
        println!("Hi! I'm {} with {} years of experience.", self.name, self.experience);
        println!("I work with: {:?}", self.languages);
    }
}

fn main() {
    let mut dev = Developer::new("Rust Developer".to_string(), 5);
    dev.add_language("Rust".to_string());
    dev.add_language("WebAssembly".to_string());

    dev.introduce();

    println!("\\n🚀 APEX.BUILD: Where Rust meets AI!");
    println!("- Claude for architectural insights");
    println!("- GPT-4 for code generation");
    println!("- Gemini for quick explanations");
    println!("- All in a stunning cyberpunk interface!");
}
`,
    runCommand: 'cargo run',
    buildCommand: 'cargo build',
    testCommand: 'cargo test',
  },
}

export const getTheme = (themeId: string): Theme => {
  return themes[themeId] || themes.cyberpunk
}

export const getLanguageConfig = (language: string) => {
  return languageConfigs[language as keyof typeof languageConfigs] || languageConfigs.typescript
}