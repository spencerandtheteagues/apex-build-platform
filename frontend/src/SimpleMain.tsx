import React from 'react'
import ReactDOM from 'react-dom/client'
import { FixedApp } from './FixedApp'

console.log('🚀 APEX.BUILD Fixed Version Loading...');

// Create root and render
const root = ReactDOM.createRoot(document.getElementById('root')!);

// Make app globally accessible for navigation
let appInstance: any = null;

root.render(
  <React.StrictMode>
    <FixedApp ref={(ref) => {
      appInstance = ref;
      // @ts-ignore
      window.ReactApp = ref;
    }} />
  </React.StrictMode>
);

console.log('✅ APEX.BUILD Fixed Version Loaded!');