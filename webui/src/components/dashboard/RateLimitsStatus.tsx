import { useEffect, useState, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import {
  Gauge,
  RefreshCw,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Loader2,
  ChevronDown,
  ChevronRight,
  Zap,
} from 'lucide-react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { cn } from '@/lib/utils'

interface CredentialStatus {
  id: string
  provider: string
  label: string
  status: 'available' | 'cooling_down' | 'disabled'
  cooldown_until?: string
  cooldown_remaining?: string
  requests_today: number
  last_used?: string
}

interface RateLimitsSummary {
  total: number
  available: number
  cooling_down: number
  disabled: number
  next_recovery_in?: string
  credentials?: CredentialStatus[]
}

export function RateLimitsStatus() {
  const { mgmtKey, mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()

  const [data, setData] = useState<RateLimitsSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [detailsOpen, setDetailsOpen] = useState(false)

  const isRunning = status?.running ?? false

  const loadStats = useCallback(async () => {
    if (!isRunning) return
    try {
      setLoading(true)
      const result = await mgmtFetch('/v0/management/rate-limits/summary')
      setData(result)
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    } finally {
      setLoading(false)
    }
  }, [mgmtFetch, showToast, isRunning])

  const handleResetCooldown = async (id?: string) => {
    try {
      const body = id ? { auth_id: id } : {}
      await mgmtFetch('/v0/management/auth/reset-cooldown', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      await loadStats()
      showToast(id ? 'Cooldown reset for credential' : 'All cooldowns reset', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  useEffect(() => {
    if (mgmtKey && isRunning) {
      loadStats()
      const interval = setInterval(loadStats, 10000) // Refresh every 10s for rate limits
      return () => clearInterval(interval)
    } else if (!isRunning) {
      setData(null)
    }
  }, [mgmtKey, isRunning, loadStats])

  if (!mgmtKey) return null

  const availablePercent = data && data.total > 0 ? (data.available / data.total) * 100 : 0

  return (
    <div className="bg-gradient-to-b from-zinc-900 to-black border border-orange-500/40 rounded-lg shadow-2xl shadow-orange-500/10 overflow-hidden">
      {/* Header */}
      <div className="bg-gradient-to-r from-orange-950/80 via-orange-900/60 to-orange-950/80 border-b border-orange-500/40 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Gauge className={cn('h-5 w-5', isRunning ? 'text-orange-400' : 'text-muted')} />
            <span className="text-sm font-mono font-bold text-orange-300 uppercase tracking-widest">
              Rate Limits
            </span>
            {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-orange-500/60" />}
          </div>
          <div className="flex items-center gap-2">
            {data && data.cooling_down > 0 && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => handleResetCooldown()}
                className="gap-1.5 text-xs font-mono bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border border-amber-500/30"
              >
                <Zap className="h-3 w-3" />
                RESET ALL
              </Button>
            )}
            <Button
              size="sm"
              variant="ghost"
              onClick={loadStats}
              disabled={!isRunning}
              className="gap-1.5 text-xs font-mono bg-orange-500/10 hover:bg-orange-500/20 text-orange-300 border border-orange-500/30"
            >
              <RefreshCw className="h-3 w-3" />
              REFRESH
            </Button>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="p-4 space-y-4">
        {!isRunning && (
          <div className="py-8 text-center text-muted-foreground border border-dashed border-orange-500/20 rounded bg-black/20">
            <p className="text-sm uppercase tracking-widest font-mono">Engine Offline</p>
            <p className="text-xs mt-1">Start the proxy engine to view rate limit status</p>
          </div>
        )}

        {isRunning && loading && !data && (
          <div className="py-8 flex justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-orange-500/60" />
          </div>
        )}

        {isRunning && data && (
          <>
            {/* Summary Stats */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <StatCard
                label="Total"
                value={data.total.toString()}
                icon={<Gauge className="h-4 w-4" />}
              />
              <StatCard
                label="Available"
                value={data.available.toString()}
                icon={<CheckCircle2 className="h-4 w-4" />}
                accent={data.available > 0 ? 'green' : 'red'}
              />
              <StatCard
                label="Cooling Down"
                value={data.cooling_down.toString()}
                icon={<Clock className="h-4 w-4" />}
                accent={data.cooling_down > 0 ? 'amber' : undefined}
              />
              <StatCard
                label="Disabled"
                value={data.disabled.toString()}
                icon={<XCircle className="h-4 w-4" />}
                accent={data.disabled > 0 ? 'red' : undefined}
              />
            </div>

            {/* Availability Bar */}
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs font-mono">
                <span className="text-orange-300/60 uppercase">Availability</span>
                <span className={cn(
                  'font-bold',
                  availablePercent >= 50 ? 'text-green-400' :
                  availablePercent > 0 ? 'text-amber-400' : 'text-red-400'
                )}>
                  {availablePercent.toFixed(0)}%
                </span>
              </div>
              <div className="h-3 w-full overflow-hidden rounded-full bg-black/60 border border-orange-500/20">
                <div className="h-full flex">
                  <div
                    className="h-full bg-green-500/70 transition-all"
                    style={{ width: `${(data.available / Math.max(data.total, 1)) * 100}%` }}
                  />
                  <div
                    className="h-full bg-amber-500/70 transition-all"
                    style={{ width: `${(data.cooling_down / Math.max(data.total, 1)) * 100}%` }}
                  />
                  <div
                    className="h-full bg-red-500/70 transition-all"
                    style={{ width: `${(data.disabled / Math.max(data.total, 1)) * 100}%` }}
                  />
                </div>
              </div>
              <div className="flex items-center justify-center gap-4 text-xs font-mono text-muted-foreground">
                <span className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-green-500/70" />
                  Available
                </span>
                <span className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-amber-500/70" />
                  Cooling
                </span>
                <span className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-red-500/70" />
                  Disabled
                </span>
              </div>
            </div>

            {/* Next Recovery */}
            {data.next_recovery_in && (
              <div className="flex items-center justify-center gap-2 py-2 px-3 bg-amber-500/10 border border-amber-500/20 rounded text-xs font-mono text-amber-300">
                <Clock className="h-3 w-3" />
                Next credential recovers in: {data.next_recovery_in}
              </div>
            )}

            {/* Credential Details Collapsible */}
            {data.credentials && data.credentials.length > 0 && (
              <div className="border border-orange-500/30 rounded bg-black/40">
                <button
                  onClick={() => setDetailsOpen(!detailsOpen)}
                  className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-orange-500/10 transition-colors"
                >
                  {detailsOpen ? (
                    <ChevronDown className="h-4 w-4 text-orange-400" />
                  ) : (
                    <ChevronRight className="h-4 w-4 text-orange-400" />
                  )}
                  <span className="text-xs font-mono text-orange-300 uppercase tracking-wider">
                    Credential Details
                  </span>
                  <span className="ml-auto text-xs font-mono bg-orange-500/20 text-orange-300 px-2 py-0.5 rounded">
                    {data.credentials.length}
                  </span>
                </button>
                {detailsOpen && (
                  <div className="px-3 pb-3">
                    <div className="bg-black/60 border border-orange-500/20 rounded max-h-64 overflow-auto">
                      <div className="divide-y divide-orange-500/10">
                        {data.credentials.map((cred) => (
                          <div key={cred.id} className="px-3 py-2 hover:bg-orange-500/5">
                            <div className="flex items-center justify-between gap-2">
                              <div className="flex items-center gap-2 flex-1 min-w-0">
                                <StatusIcon status={cred.status} />
                                <div className="flex flex-col min-w-0">
                                  <span className="text-xs font-mono text-orange-200 truncate">
                                    {cred.label || cred.id.substring(0, 8)}
                                  </span>
                                  <span className="text-xs font-mono text-muted-foreground capitalize">
                                    {cred.provider}
                                  </span>
                                </div>
                              </div>
                              <div className="flex items-center gap-2 shrink-0">
                                {cred.cooldown_remaining && (
                                  <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300">
                                    {cred.cooldown_remaining}
                                  </span>
                                )}
                                <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-300">
                                  {cred.requests_today} today
                                </span>
                                {cred.status === 'cooling_down' && (
                                  <Button
                                    size="sm"
                                    variant="ghost"
                                    onClick={() => handleResetCooldown(cred.id)}
                                    className="h-6 px-2 text-xs font-mono bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border border-amber-500/30"
                                  >
                                    <Zap className="h-3 w-3" />
                                  </Button>
                                )}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function StatusIcon({ status }: { status: 'available' | 'cooling_down' | 'disabled' }) {
  switch (status) {
    case 'available':
      return <CheckCircle2 className="h-4 w-4 text-green-400" />
    case 'cooling_down':
      return <Clock className="h-4 w-4 text-amber-400" />
    case 'disabled':
      return <XCircle className="h-4 w-4 text-red-400" />
    default:
      return <AlertCircle className="h-4 w-4 text-muted-foreground" />
  }
}

function StatCard({
  label,
  value,
  icon,
  accent,
}: {
  label: string
  value: string
  icon: React.ReactNode
  accent?: 'green' | 'amber' | 'red'
}) {
  let valueColor = 'text-orange-200'
  if (accent === 'green') valueColor = 'text-green-400'
  if (accent === 'amber') valueColor = 'text-amber-400'
  if (accent === 'red') valueColor = 'text-red-400'

  return (
    <div className="bg-black/40 border border-orange-500/30 rounded p-3">
      <div className="flex items-center gap-1.5 mb-1">
        <span className="text-orange-400">{icon}</span>
        <span className="text-xs font-mono uppercase text-orange-500/60">{label}</span>
      </div>
      <div className={cn('text-lg font-mono font-bold', valueColor)}>{value}</div>
    </div>
  )
}
