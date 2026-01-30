// APEX Component Library - Stub for ApexIDEDemo
// These are placeholder components for the demo

import React from 'react'

export const APEXButton: React.FC<any> = ({ children, ...props }) => (
  <button {...props}>{children}</button>
)

export const APEXCard: React.FC<any> = ({ children, ...props }) => (
  <div {...props}>{children}</div>
)

export const APEXInput: React.FC<any> = (props) => <input {...props} />

export const APEXTitle: React.FC<any> = ({ children, ...props }) => (
  <h1 {...props}>{children}</h1>
)

export const APEXNav: React.FC<any> = ({ children, ...props }) => (
  <nav {...props}>{children}</nav>
)

export const APEXLoading: React.FC<any> = () => (
  <div className="animate-pulse">Loading...</div>
)

export const APEXParticleBackground: React.FC<any> = () => null

interface ThemeContextValue {
  theme: string
  setTheme: (theme: string) => void
}

const ThemeContext = React.createContext<ThemeContextValue>({
  theme: 'dark',
  setTheme: () => {},
})

export const APEXThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [theme, setTheme] = React.useState('dark')
  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useAPEXTheme = () => React.useContext(ThemeContext)

export const APEX_TOKENS = {
  colors: {
    primary: '#ef4444',
    secondary: '#1f2937',
    accent: '#06b6d4',
    text: '#f9fafb',
    background: '#111827',
  },
  spacing: {
    xs: '0.25rem',
    sm: '0.5rem',
    md: '1rem',
    lg: '1.5rem',
    xl: '2rem',
  },
}
