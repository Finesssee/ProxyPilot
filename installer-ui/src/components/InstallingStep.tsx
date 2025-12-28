import { motion } from 'framer-motion'

interface InstallingStepProps {
  progress: number
  status: string
}

export function InstallingStep({ progress, status }: InstallingStepProps) {
  const radius = 52
  const circumference = 2 * Math.PI * radius
  const strokeDashoffset = circumference - (circumference * progress) / 100

  return (
    <div className="installing-step">
      {/* Progress Circle */}
      <motion.div
        className="progress-circle-container"
        initial={{ scale: 0.8, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{ duration: 0.5 }}
      >
        <svg className="progress-circle" viewBox="0 0 120 120">
          <circle
            className="progress-circle-bg"
            cx="60"
            cy="60"
            r={radius}
          />
          <motion.circle
            className="progress-circle-fill"
            cx="60"
            cy="60"
            r={radius}
            strokeDasharray={circumference}
            strokeDashoffset={strokeDashoffset}
            initial={{ strokeDashoffset: circumference }}
            animate={{ strokeDashoffset }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          />
        </svg>
        <div className="progress-percent">
          {Math.round(progress)}<span>%</span>
        </div>
      </motion.div>

      {/* Progress Info */}
      <motion.div
        className="progress-info"
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.2, duration: 0.4 }}
      >
        <h2 className="installing-title">Installing ProxyPilot</h2>

        {/* Progress Bar */}
        <div className="progress-bar-container">
          <motion.div
            className="progress-bar-fill"
            initial={{ width: 0 }}
            animate={{ width: `${progress}%` }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          />
        </div>

        {/* Status */}
        <motion.p
          className="status-text"
          key={status}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.2 }}
        >
          {status}
        </motion.p>
      </motion.div>
    </div>
  )
}
