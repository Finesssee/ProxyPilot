import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

// Type declarations for Go/WebView2 bridge
declare global {
  interface Window {
    installer: {
      startInstall: (options?: { installPath?: string }) => void
      launch: () => void
      close: () => void
    }
    setProgress: (percent: number, status: string) => void
  }
}

// Store progress callback for Go to call
let progressCallback: ((percent: number, status: string) => void) | null = null

export function setProgressCallback(callback: (percent: number, status: string) => void) {
  progressCallback = callback
}

// Expose to Go backend
window.setProgress = (percent: number, status: string) => {
  if (progressCallback) {
    progressCallback(percent, status)
  }
}

// Mock installer for development (will be replaced by WebView2 in production)
if (!window.installer) {
  window.installer = {
    startInstall: () => {
      console.log('Mock: Starting installation...')
      let progress = 0
      const steps = [
        'Preparing installation...',
        'Extracting files...',
        'Installing components...',
        'Configuring ProxyPilot...',
        'Finishing up...'
      ]
      const interval = setInterval(() => {
        progress += Math.random() * 15 + 5
        if (progress >= 100) {
          progress = 100
          clearInterval(interval)
        }
        const stepIndex = Math.min(Math.floor(progress / 20), steps.length - 1)
        window.setProgress(Math.round(progress), steps[stepIndex])
      }, 500)
    },
    launch: () => {
      console.log('Mock: Launching ProxyPilot...')
    },
    close: () => {
      console.log('Mock: Closing installer...')
    }
  }
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
