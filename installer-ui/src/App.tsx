import { useState, useEffect, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { WelcomeStep } from './components/WelcomeStep'
import { InstallingStep } from './components/InstallingStep'
import { CompleteStep } from './components/CompleteStep'
import { setProgressCallback } from './main'

type InstallStep = 'welcome' | 'installing' | 'complete'

function App() {
  const [step, setStep] = useState<InstallStep>('welcome')
  const [progress, setProgress] = useState(0)
  const [status, setStatus] = useState('Preparing installation...')

  const handleProgress = useCallback((percent: number, statusText: string) => {
    setProgress(percent)
    setStatus(statusText)
    if (percent >= 100) {
      setTimeout(() => setStep('complete'), 800)
    }
  }, [])

  useEffect(() => {
    setProgressCallback(handleProgress)
  }, [handleProgress])

  const handleInstall = () => {
    setStep('installing')
    setProgress(0)
    setStatus('Preparing installation...')
    window.installer.startInstall()
  }

  const handleLaunch = () => {
    window.installer.launch()
  }

  const pageVariants = {
    initial: {
      opacity: 0,
      y: 20
    },
    animate: {
      opacity: 1,
      y: 0,
      transition: {
        duration: 0.4,
        ease: [0.25, 0.1, 0.25, 1]
      }
    },
    exit: {
      opacity: 0,
      y: -20,
      transition: {
        duration: 0.3,
        ease: [0.25, 0.1, 0.25, 1]
      }
    }
  }

  return (
    <div className="app-container">
      <main className="main-content">
        <AnimatePresence mode="wait">
          {step === 'welcome' && (
            <motion.div
              key="welcome"
              variants={pageVariants}
              initial="initial"
              animate="animate"
              exit="exit"
              className="step-container"
            >
              <WelcomeStep onInstall={handleInstall} />
            </motion.div>
          )}

          {step === 'installing' && (
            <motion.div
              key="installing"
              variants={pageVariants}
              initial="initial"
              animate="animate"
              exit="exit"
              className="step-container"
            >
              <InstallingStep progress={progress} status={status} />
            </motion.div>
          )}

          {step === 'complete' && (
            <motion.div
              key="complete"
              variants={pageVariants}
              initial="initial"
              animate="animate"
              exit="exit"
              className="step-container"
            >
              <CompleteStep onLaunch={handleLaunch} />
            </motion.div>
          )}
        </AnimatePresence>
      </main>

      {/* Window Controls */}
      <div className="window-controls">
        <button
          className="window-btn"
          onClick={() => window.installer.close()}
          aria-label="Close"
        >
          <svg viewBox="0 0 10 10" fill="none">
            <path d="M1 1L9 9M9 1L1 9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
          </svg>
        </button>
      </div>
    </div>
  )
}

export default App
