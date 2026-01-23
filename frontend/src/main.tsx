// APEX.BUILD Frontend Entry Point
// Cyberpunk cloud development platform initialization

import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './styles/globals.css'

// Initialize Monaco Editor worker
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'
import cssWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker'
import htmlWorker from 'monaco-editor/esm/vs/language/html/html.worker?worker'
import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker'

// Set up Monaco Editor environment
self.MonacoEnvironment = {
  getWorker(_, label) {
    if (label === 'json') {
      return new jsonWorker()
    }
    if (label === 'css' || label === 'scss' || label === 'less') {
      return new cssWorker()
    }
    if (label === 'html' || label === 'handlebars' || label === 'razor') {
      return new htmlWorker()
    }
    if (label === 'typescript' || label === 'javascript') {
      return new tsWorker()
    }
    return new editorWorker()
  },
}

// Console welcome message
console.log(`
🚀 APEX.BUILD v1.0.0
⚡ Cyberpunk Cloud Development Platform

🔥 Features:
   • Multi-AI Integration (Claude + GPT-4 + Gemini)
   • Real-time Collaboration
   • Intelligent Code Editor
   • Cloud Execution Environment
   • Beautiful Cyberpunk UI

💻 Built with cutting-edge tech:
   • React 18 + TypeScript
   • Monaco Editor
   • WebSocket Collaboration
   • Go Backend with PostgreSQL
   • Docker Containerization

🌐 Website: https://apex.build
📧 Support: support@apex.build

Welcome to the future of development! 🌌
`)

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)