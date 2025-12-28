import React, { useState, useEffect, useMemo } from 'react'
import { useProxyContext } from '@/hooks/useProxyContext'

// Provider configuration matching ProviderLogins
const providers = [
  { id: 'claude', name: 'Claude' },
  { id: 'gemini', name: 'Gemini' },
  { id: 'codex', name: 'Codex' },
  { id: 'qwen', name: 'Qwen' },
  { id: 'anthropic', name: 'Anthropic' },
] as const

type ProviderId = (typeof providers)[number]['id']

interface StatusBarProps {
  version?: string
  requestCount?: number
  providerStatus?: Record<ProviderId, boolean>
}

// Format uptime as HH:MM:SS
function formatUptime(seconds: number): string {
  const hrs = Math.floor(seconds / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  return `${hrs.toString().padStart(2, '0')}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
}

// Format number with commas
function formatNumber(num: number): string {
  return num.toLocaleString()
}

export const StatusBar: React.FC<StatusBarProps> = ({
  version = 'v0.1.0',
  requestCount = 0,
  providerStatus,
}) => {
  const { status, authFiles } = useProxyContext()
  const [uptime, setUptime] = useState(0)
  const [startTime] = useState(() => Date.now())

  // Derive provider status from auth files or use provided status
  const activeProviders = useMemo(() => {
    if (providerStatus) return providerStatus
    return {
      claude: authFiles.some(f => f.toLowerCase().includes('claude')),
      gemini: authFiles.some(f => f.toLowerCase().includes('gemini')),
      codex: authFiles.some(f => f.toLowerCase().includes('codex')),
      qwen: authFiles.some(f => f.toLowerCase().includes('qwen')),
      anthropic: authFiles.some(f => f.toLowerCase().includes('anthropic')),
    }
  }, [providerStatus, authFiles])

  // Uptime counter - updates every second
  useEffect(() => {
    const interval = setInterval(() => {
      setUptime(Math.floor((Date.now() - startTime) / 1000))
    }, 1000)
    return () => clearInterval(interval)
  }, [startTime])

  const isOnline = status?.running ?? false
  const port = status?.port ?? 8317

  return (
    <div className="instrument-panel">
      {/* Provider Indicator Lights */}
      <div className="instrument-segment instrument-segment--providers" title="Provider Status">
        <div className="provider-lights">
          {providers.map((provider) => (
            <span
              key={provider.id}
              className={`provider-light ${activeProviders[provider.id] ? 'provider-light--active' : 'provider-light--inactive'}`}
              title={`${provider.name}: ${activeProviders[provider.id] ? 'Connected' : 'Offline'}`}
            />
          ))}
        </div>
      </div>

      {/* Port Display */}
      <div className="instrument-segment" title="Proxy Port">
        <span className="segment-label">PORT</span>
        <span className="segment-value segment-value--accent">{port}</span>
      </div>

      {/* Uptime Counter */}
      <div className="instrument-segment" title="Session Uptime">
        <span className="segment-label">UPTIME</span>
        <span className="segment-value segment-value--uptime">{formatUptime(uptime)}</span>
      </div>

      {/* Request Counter */}
      <div className="instrument-segment" title="Total Requests">
        <span className="segment-label">REQ</span>
        <span className="segment-value">{formatNumber(requestCount)}</span>
      </div>

      {/* Version */}
      <div className="instrument-segment instrument-segment--version" title="ProxyPilot Version">
        <span className="segment-value segment-value--muted">{version}</span>
      </div>

      {/* Status Indicator */}
      <div className="instrument-segment instrument-segment--status" title="Proxy Status">
        <span className={`status-indicator ${isOnline ? 'status-indicator--online' : 'status-indicator--offline'}`}>
          <span className="status-dot" />
          <span className="status-text">{isOnline ? 'READY' : 'OFFLINE'}</span>
        </span>
      </div>

      <style>{`
        .instrument-panel {
          height: 30px;
          background: linear-gradient(to bottom, var(--bg-panel), var(--bg-void));
          border-top: 1px solid var(--border-subtle);
          display: flex;
          align-items: center;
          font-family: var(--font-mono);
          font-size: 0.65rem;
          user-select: none;
        }

        .instrument-segment {
          display: flex;
          align-items: center;
          gap: 6px;
          padding: 0 12px;
          height: 100%;
          border-right: 1px solid var(--border-subtle);
          background: linear-gradient(to bottom, transparent, rgba(0, 0, 0, 0.05));
        }

        .instrument-segment:last-child {
          border-right: none;
        }

        .instrument-segment--providers {
          padding: 0 10px;
        }

        .instrument-segment--version {
          margin-left: auto;
          border-left: 1px solid var(--border-subtle);
          border-right: 1px solid var(--border-subtle);
        }

        .instrument-segment--status {
          padding: 0 14px;
        }

        .provider-lights {
          display: flex;
          align-items: center;
          gap: 5px;
        }

        .provider-light {
          width: 7px;
          height: 7px;
          border-radius: 50%;
          transition: all 0.3s ease;
        }

        .provider-light--active {
          background: var(--accent-glow);
          box-shadow: 0 0 6px var(--accent-glow), 0 0 10px var(--accent-primary);
        }

        .provider-light--inactive {
          background: var(--border-subtle);
          opacity: 0.6;
        }

        .segment-label {
          color: var(--text-muted);
          font-size: 0.6rem;
          letter-spacing: 0.05em;
        }

        .segment-value {
          color: var(--text-primary);
          font-weight: 500;
          letter-spacing: 0.02em;
        }

        .segment-value--accent {
          color: var(--accent-primary);
        }

        .segment-value--muted {
          color: var(--text-muted);
          font-weight: 400;
        }

        .segment-value--uptime {
          font-variant-numeric: tabular-nums;
          min-width: 5.5em;
        }

        .status-indicator {
          display: flex;
          align-items: center;
          gap: 6px;
        }

        .status-dot {
          width: 6px;
          height: 6px;
          border-radius: 50%;
          transition: all 0.3s ease;
        }

        .status-indicator--online .status-dot {
          background: var(--status-online);
          box-shadow: 0 0 6px var(--status-online);
          animation: statusPulse 2s ease-in-out infinite;
        }

        .status-indicator--offline .status-dot {
          background: var(--status-offline);
        }

        .status-indicator--online .status-text {
          color: var(--status-online);
        }

        .status-indicator--offline .status-text {
          color: var(--status-offline);
        }

        .status-text {
          font-weight: 600;
          letter-spacing: 0.08em;
        }

        @keyframes statusPulse {
          0%, 100% {
            opacity: 1;
            transform: scale(1);
          }
          50% {
            opacity: 0.7;
            transform: scale(1.15);
          }
        }

        /* Dark mode adjustments */
        .dark .instrument-segment {
          background: linear-gradient(to bottom, transparent, rgba(0, 0, 0, 0.15));
        }

        .dark .provider-light--inactive {
          background: var(--border-default);
          opacity: 0.4;
        }
      `}</style>
    </div>
  )
}

export default StatusBar
