import { motion } from 'framer-motion'

interface CompleteStepProps {
  onLaunch: () => void
}

export function CompleteStep({ onLaunch }: CompleteStepProps) {
  return (
    <div className="complete-step">
      {/* Success Icon */}
      <motion.div
        className="success-icon-container"
        initial={{ scale: 0 }}
        animate={{ scale: 1 }}
        transition={{
          type: 'spring',
          stiffness: 200,
          damping: 15,
          delay: 0.1
        }}
      >
        <div className="success-icon">
          <svg viewBox="0 0 24 24" fill="none">
            <motion.path
              d="M5 13l4 4L19 7"
              stroke="currentColor"
              strokeWidth="3"
              strokeLinecap="round"
              strokeLinejoin="round"
              initial={{ pathLength: 0 }}
              animate={{ pathLength: 1 }}
              transition={{ duration: 0.4, delay: 0.3 }}
            />
          </svg>
        </div>
      </motion.div>

      {/* Success Content */}
      <motion.div
        className="success-content"
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.4, duration: 0.4 }}
      >
        <h2 className="success-title">Installation Complete</h2>
        <p className="success-message">
          ProxyPilot has been installed successfully and is ready to use.
        </p>
      </motion.div>

      {/* Launch Button */}
      <motion.button
        className="launch-btn"
        onClick={onLaunch}
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.5, duration: 0.4 }}
        whileHover={{ scale: 1.02 }}
        whileTap={{ scale: 0.98 }}
      >
        <svg viewBox="0 0 20 20" fill="none">
          <path d="M4 10h12m0 0l-4-4m4 4l-4 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
        Launch ProxyPilot
      </motion.button>

      {/* Post Install Info */}
      <motion.div
        className="post-install-info"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.6, duration: 0.4 }}
      >
        <div className="info-icon">
          <svg viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.5"/>
            <path d="M8 5v3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
            <circle cx="8" cy="11" r="0.75" fill="currentColor"/>
          </svg>
        </div>
        <p>ProxyPilot will start automatically and run in the system tray.</p>
      </motion.div>
    </div>
  )
}
