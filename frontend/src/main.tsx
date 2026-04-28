// APEX-BUILD frontend entry point

import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './styles/globals.css'

console.log(`
APEX-BUILD v1.0.0
Multi-agent AI app builder

Core surfaces:
   - 9 specialized agents across flagship and open-weight models
   - BYOK routing, live cost tracking, and MCP connectors
   - Monaco editor, cloud execution, GitHub export, and deploy controls
   - Production-grade review, testing, secrets, and collaboration workflows

Website: https://apex-build.dev
Support: support@apex-build.dev
`)

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
