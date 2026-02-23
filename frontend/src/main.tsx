// APEX.BUILD Frontend Entry Point
// Cyberpunk cloud development platform initialization

import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './styles/globals.css'

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
