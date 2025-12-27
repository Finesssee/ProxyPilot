import { useEffect, useState, useCallback } from 'react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Radar, Settings, Download, ExternalLink, Check, AlertCircle, Loader2 } from 'lucide-react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { AgentSetup } from './AgentSetup'

interface Integration {
  id: string
  name?: string
  detected: boolean
  binary_path?: string
  config_path?: string
}

export function Integrations() {
  const { mgmtKey, mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()
  const [integrations, setIntegrations] = useState<Integration[]>([])
  const [loading, setLoading] = useState(false)
  const [configuring, setConfiguring] = useState<string | null>(null)
  const [scanKey, setScanKey] = useState(0)
  const isRunning = status?.running ?? false

  const fetchIntegrations = useCallback(async () => {
    if (!mgmtKey || !isRunning) return
    setLoading(true)
    setScanKey(prev => prev + 1)
    try {
      const data = await mgmtFetch('/v0/management/integrations/status')
      setIntegrations(data.integrations || [])
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        console.error('Failed to fetch integrations:', e)
      }
    }
    setLoading(false)
  }, [mgmtKey, mgmtFetch, isRunning])

  const configureIntegration = async (integrationId: string) => {
    setConfiguring(integrationId)
    try {
      await mgmtFetch(`/v0/management/integrations/${integrationId}/configure`, {
        method: 'POST',
      })
      await fetchIntegrations()
      showToast(`Configured ${integrationId}`, 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
    setConfiguring(null)
  }

  useEffect(() => {
    if (mgmtKey && isRunning) {
      fetchIntegrations()
    } else if (!isRunning) {
      setIntegrations([])
    }
  }, [mgmtKey, isRunning, fetchIntegrations])

  const truncatePath = (path: string, maxLen: number = 35) => {
    if (path.length <= maxLen) return path
    const start = path.slice(0, 15)
    const end = path.slice(-17)
    return `${start}...${end}`
  }

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-[var(--border-default)] bg-[var(--bg-panel)] shadow-xl overflow-hidden">
        {/* Ground Control Header */}
        <div className="flex items-center gap-2 px-4 py-3 bg-[var(--bg-elevated)] border-b border-[var(--border-default)]">
          <div className="relative">
            <Radar className={`h-4 w-4 text-[var(--accent-primary)] ${loading || isMgmtLoading ? 'animate-spin' : ''}`} />
            {(loading || isMgmtLoading) && (
              <div className="absolute inset-0 rounded-full border border-[var(--accent-primary)] animate-ping opacity-50" />
            )}
          </div>
          <span className="font-mono text-xs font-bold uppercase tracking-widest text-[var(--text-secondary)]">
            Ground Control
          </span>
          {isMgmtLoading && <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />}
          <div className="flex-1" />

          {/* Scan Button */}
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                onClick={fetchIntegrations}
                disabled={loading || !isRunning}
                className={`
                  relative flex items-center gap-1.5 px-3 py-1.5 rounded
                  font-mono text-[10px] uppercase tracking-wider font-bold
                  border transition-all duration-150
                  ${loading
                    ? 'border-[var(--status-processing)] bg-[var(--status-processing)]/10 text-[var(--status-processing)]'
                    : !isRunning
                      ? 'border-[var(--border-subtle)] bg-transparent text-[var(--text-muted)] cursor-not-allowed opacity-50'
                      : 'border-[var(--border-default)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)] hover:shadow-[0_0_10px_var(--accent-primary)]'
                  }
                  active:translate-y-0.5 cursor-pointer
                  overflow-hidden
                `}
              >
                {/* Scan sweep effect */}
                {loading && (
                  <div
                    className="absolute inset-0 bg-gradient-to-r from-transparent via-[var(--status-processing)]/30 to-transparent"
                    style={{
                      animation: 'scan-sweep 1s ease-in-out infinite',
                    }}
                  />
                )}
                <Radar className={`h-3 w-3 relative z-10 ${loading ? 'animate-spin' : ''}`} />
                <span className="relative z-10">{loading ? 'Scanning' : 'Scan'}</span>
              </button>
            </TooltipTrigger>
            <TooltipContent>Scan for CLI agent integrations</TooltipContent>
          </Tooltip>
        </div>

        {/* Station Grid */}
        <div className="p-4 relative">
          {/* Radar sweep overlay during scan */}
          {loading && (
            <div
              className="absolute inset-0 pointer-events-none z-10 overflow-hidden"
              style={{
                background: 'linear-gradient(90deg, transparent 0%, var(--accent-primary) 50%, transparent 100%)',
                opacity: 0.08,
                animation: 'radar-scan-horizontal 1.5s ease-in-out',
              }}
            />
          )}

          {/* Empty State */}
          {!isRunning ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <div className="font-mono text-sm text-[var(--text-muted)] uppercase tracking-wider mb-1">
                ⚠️ Engine Offline
              </div>
              <div className="text-xs text-[var(--text-muted)]">
                Start the proxy engine to manage integrations
              </div>
            </div>
          ) : integrations.length === 0 && !loading && (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <div className="relative mb-4">
                <div className="w-16 h-16 rounded-full border-2 border-dashed border-[var(--border-subtle)] flex items-center justify-center">
                  <Radar className="h-6 w-6 text-[var(--text-muted)]" />
                </div>
              </div>
              <div className="font-mono text-sm text-[var(--text-muted)] uppercase tracking-wider mb-1">
                No Signals Detected
              </div>
              <div className="text-xs text-[var(--text-muted)]">
                Click SCAN to search for CLI agents
              </div>
            </div>
          )}

          {/* Integration Cards Grid */}
          <div
            key={scanKey}
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
          >
            {integrations.map((integration, index) => (
              <div
                key={integration.id}
                className={`
                  group relative rounded-lg border overflow-hidden
                  transition-all duration-300
                  ${integration.detected
                    ? 'border-[var(--status-online)]/50 bg-gradient-to-b from-[var(--bg-elevated)] to-[var(--bg-panel)]'
                    : 'border-[var(--border-subtle)] bg-[var(--bg-void)]'
                  }
                  hover:border-[var(--accent-primary)] hover:shadow-[0_0_20px_var(--accent-primary)/15]
                  hover:translate-y-[-2px]
                `}
                style={{
                  animation: `station-fade-in 0.4s ease-out ${index * 0.08}s both`,
                }}
              >
                {/* Station Header Badge */}
                <div className={`
                  flex items-center gap-2 px-3 py-2 border-b
                  ${integration.detected
                    ? 'border-[var(--status-online)]/30 bg-[var(--status-online)]/5'
                    : 'border-[var(--border-subtle)] bg-[var(--bg-elevated)]'
                  }
                `}>
                  {/* Status Indicator */}
                  <div className="relative">
                    <div className={`
                      w-3 h-3 rounded-full transition-all duration-300
                      ${integration.detected
                        ? 'bg-[var(--status-online)] shadow-[0_0_8px_var(--status-online)]'
                        : 'border-2 border-[var(--text-muted)] bg-transparent'
                      }
                    `}>
                      {integration.detected && (
                        <div
                          className="absolute inset-0 rounded-full bg-[var(--status-online)]"
                          style={{
                            animation: 'status-glow 2s ease-in-out infinite',
                          }}
                        />
                      )}
                    </div>
                  </div>

                  {/* Integration Name */}
                  <span className={`
                    font-mono text-xs font-bold uppercase tracking-wider flex-1
                    ${integration.detected ? 'text-[var(--text-primary)]' : 'text-[var(--text-muted)]'}
                  `}>
                    {integration.name || integration.id}
                  </span>
                </div>

                {/* Station Body */}
                <div className="p-3 space-y-3">
                  {/* Status Text */}
                  <div className={`
                    flex items-center gap-2 font-mono text-xs uppercase tracking-wider
                    ${integration.detected ? 'text-[var(--status-online)]' : 'text-[var(--text-muted)]'}
                  `}>
                    {integration.detected ? (
                      <>
                        <Check className="h-3 w-3" />
                        <span>Detected</span>
                      </>
                    ) : (
                      <>
                        <AlertCircle className="h-3 w-3" />
                        <span>Not Detected</span>
                      </>
                    )}
                  </div>

                  {/* Path Info (when detected) */}
                  {integration.detected && (
                    <div className="space-y-1.5 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
                      {integration.binary_path && (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <div className="flex items-center gap-1.5 text-[10px] font-mono text-[var(--text-muted)] truncate cursor-help">
                              <ExternalLink className="h-2.5 w-2.5 shrink-0" />
                              <span className="truncate">{truncatePath(integration.binary_path)}</span>
                            </div>
                          </TooltipTrigger>
                          <TooltipContent side="bottom" className="max-w-xs">
                            <div className="font-mono text-xs break-all">{integration.binary_path}</div>
                          </TooltipContent>
                        </Tooltip>
                      )}
                      {integration.config_path && (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <div className="flex items-center gap-1.5 text-[10px] font-mono text-[var(--text-muted)] truncate cursor-help">
                              <Settings className="h-2.5 w-2.5 shrink-0" />
                              <span className="truncate">{truncatePath(integration.config_path)}</span>
                            </div>
                          </TooltipTrigger>
                          <TooltipContent side="bottom" className="max-w-xs">
                            <div className="font-mono text-xs break-all">{integration.config_path}</div>
                          </TooltipContent>
                        </Tooltip>
                      )}
                    </div>
                  )}

                  {/* Spacer for non-detected items to maintain consistent height */}
                  {!integration.detected && (
                    <div className="h-[30px]" />
                  )}

                  {/* Action Button */}
                  <button
                    type="button"
                    onClick={() => integration.detected && configureIntegration(integration.id)}
                    disabled={!integration.detected || configuring === integration.id}
                    className={`
                      w-full flex items-center justify-center gap-2 px-3 py-2 rounded
                      font-mono text-xs font-bold uppercase tracking-wider
                      border transition-all duration-150
                      ${configuring === integration.id
                        ? 'border-[var(--status-processing)] bg-[var(--status-processing)]/10 text-[var(--status-processing)]'
                        : integration.detected
                          ? 'border-[var(--accent-primary)] bg-transparent text-[var(--accent-primary)] hover:bg-[var(--accent-primary)] hover:text-[var(--bg-panel)] hover:shadow-[0_0_15px_var(--accent-primary)]'
                          : 'border-[var(--border-subtle)] bg-transparent text-[var(--text-muted)] cursor-not-allowed opacity-50'
                      }
                      active:translate-y-0.5 cursor-pointer
                      disabled:active:translate-y-0
                    `}
                  >
                    {configuring === integration.id ? (
                      <>
                        <Settings className="h-3 w-3 animate-spin" />
                        <span>Configuring</span>
                      </>
                    ) : integration.detected ? (
                      <>
                        <Settings className="h-3 w-3" />
                        <span>Configure</span>
                      </>
                    ) : (
                      <>
                        <Download className="h-3 w-3" />
                        <span>Install</span>
                      </>
                    )}
                  </button>
                </div>

                {/* Configuring overlay effect */}
                {configuring === integration.id && (
                  <div
                    className="absolute inset-0 pointer-events-none"
                    style={{
                      background: 'linear-gradient(180deg, transparent 0%, var(--status-processing) 50%, transparent 100%)',
                      opacity: 0.1,
                      animation: 'config-scan 1s ease-in-out infinite',
                    }}
                  />
                )}
              </div>
            ))}
          </div>
        </div>

        {/* Footer Stats */}
        {integrations.length > 0 && (
          <div className="flex items-center gap-4 px-4 py-2 bg-[var(--bg-void)] border-t border-[var(--border-subtle)]">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-[var(--status-online)]" />
              <span className="font-mono text-[10px] uppercase tracking-wider text-[var(--text-muted)]">
                {integrations.filter(i => i.detected).length} Online
              </span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full border border-[var(--text-muted)]" />
              <span className="font-mono text-[10px] uppercase tracking-wider text-[var(--text-muted)]">
                {integrations.filter(i => !i.detected).length} Offline
              </span>
            </div>
          </div>
        )}
      </div>

      <AgentSetup />

      {/* Animation Keyframes */}
      <style>{`
        @keyframes station-fade-in {
          0% {
            opacity: 0;
            transform: translateY(10px) scale(0.98);
          }
          100% {
            opacity: 1;
            transform: translateY(0) scale(1);
          }
        }

        @keyframes status-glow {
          0%, 100% {
            opacity: 1;
            transform: scale(1);
          }
          50% {
            opacity: 0.6;
            transform: scale(1.3);
          }
        }

        @keyframes scan-sweep {
          0% {
            transform: translateX(-100%);
          }
          100% {
            transform: translateX(200%);
          }
        }

        @keyframes radar-scan-horizontal {
          0% {
            transform: translateX(-100%);
          }
          100% {
            transform: translateX(100%);
          }
        }

        @keyframes config-scan {
          0% {
            transform: translateY(-100%);
          }
          100% {
            transform: translateY(100%);
          }
        }
      `}</style>
    </div>
  )
}
