import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Loader2 } from 'lucide-react'
import { Toaster } from 'sonner'
import { ProxyProvider, useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import {
  EngineControl,
  ProviderLogins,
  Integrations,
  ModelMappings,
  MemoryManager,
  SemanticMemory,
  LogsViewer,
  ConfigEditor,
  RequestMonitor,
  UsageStats,
} from '@/components/dashboard'
import { Header } from '@/components/layout/Header'
import { StatusBar } from '@/components/layout/StatusBar'
import { IconRail, navigationItems } from '@/components/ui/icon-rail'

type ViewId = 'command' | 'providers' | 'routing' | 'memory' | 'logs' | 'requests' | 'analytics'

function DashboardContent() {
  const { status, isDesktop, mgmtKey, mgmtFetch, isMgmtLoading } = useProxyContext()
  const [mgmtError, setMgmtError] = useState<string | null>(null)
  const [mgmtConfig, setMgmtConfig] = useState<any>(null)
  const [authFiles, setAuthFiles] = useState<any[]>([])
  const [activeView, setActiveView] = useState<ViewId>('command')

  const isRunning = status?.running ?? false

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key >= '1' && e.key <= '7') {
        e.preventDefault()
        const index = parseInt(e.key, 10) - 1
        const item = navigationItems[index]
        if (item) {
          setActiveView(item.id as ViewId)
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const loadMgmtConfig = async () => {
    try {
      const cfg = await mgmtFetch('/v0/management/config')
      setMgmtConfig(cfg)
    } catch (e) {
      if (e instanceof EngineOfflineError) {
        setMgmtError('Engine Offline')
      } else {
        setMgmtError(e instanceof Error ? e.message : String(e))
      }
    }
  }

  const toggleDebug = async () => {
    try {
      const cur = await mgmtFetch('/v0/management/debug')
      await mgmtFetch('/v0/management/debug', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: !cur.value }),
      })
      await loadMgmtConfig()
    } catch (e) {
      console.error(e)
    }
  }

  const saveRetry = async (value: number) => {
    try {
      await mgmtFetch('/v0/management/request-retry', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value }),
      })
      await loadMgmtConfig()
    } catch (e) {
      console.error(e)
    }
  }

  const saveMaxRetry = async (value: number) => {
    try {
      await mgmtFetch('/v0/management/max-retry-interval', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value }),
      })
      await loadMgmtConfig()
    } catch (e) {
      console.error(e)
    }
  }

  const loadAuthFiles = async () => {
    try {
      const res = await mgmtFetch('/v0/management/auth-files')
      setAuthFiles(res.files || [])
    } catch (e) {
      console.error('Load auth files error:', e)
    }
  }

  const resetCooldown = async (authId?: string) => {
    try {
      await mgmtFetch('/v0/management/auth/reset-cooldown', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(authId ? { auth_id: authId } : {}),
      })
      await loadAuthFiles()
    } catch (e) {
      console.error(e)
    }
  }

  useEffect(() => {
    if (!mgmtKey || !isRunning) {
      if (!isRunning) {
        setMgmtError(null)
      }
      return
    }
    ; (async () => {
      try {
        await loadMgmtConfig()
        await loadAuthFiles()
      } catch (e) {
        if (e instanceof EngineOfflineError) {
          setMgmtError('Engine Offline')
        } else {
          setMgmtError(e instanceof Error ? e.message : String(e))
        }
      }
    })()
  }, [mgmtKey, isRunning])

  const debugOn = !!mgmtConfig?.debug
  const retryVal =
    mgmtConfig?.['request-retry'] ??
    mgmtConfig?.request_retry ??
    mgmtConfig?.requestRetry ??
    0
  const maxRetryVal =
    mgmtConfig?.['max-retry-interval'] ??
    mgmtConfig?.max_retry_interval ??
    mgmtConfig?.maxRetryInterval ??
    0

  // Render view content based on activeView
  const renderViewContent = () => {
    switch (activeView) {
      case 'command':
        return (
          <div className="space-y-6">
            <div className="grid gap-6 lg:grid-cols-2">
              <EngineControl />
            </div>
            <Integrations />
          </div>
        )

      case 'providers':
        return (
          <div className="space-y-6">
            <ProviderLogins />

            {!isDesktop && !mgmtKey && (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key missing. Start ProxyPilot from the tray app to inject a local key,
                or set `MANAGEMENT_PASSWORD` before loading this page.
              </div>
            )}

            {mgmtError && mgmtError !== 'Engine Offline' && (
              <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
                Management error: {mgmtError}
              </div>
            )}

            {mgmtKey && isRunning && (
              <div className="grid gap-6 lg:grid-cols-2">
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                  <CardHeader className="pb-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="text-lg">Quick Config</CardTitle>
                        <CardDescription>Management API settings</CardDescription>
                      </div>
                      {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-muted-foreground">Debug</span>
                      <div className="flex items-center gap-2">
                        <Badge variant={debugOn ? 'default' : 'secondary'}>
                          {debugOn ? 'On' : 'Off'}
                        </Badge>
                        <Button size="sm" variant="outline" onClick={() => toggleDebug().catch((e) => console.error(e))}>
                          Toggle
                        </Button>
                      </div>
                    </div>
                    <Separator className="bg-border/50" />
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm text-muted-foreground">Request retry</span>
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          min={0}
                          max={10}
                          className="w-24 rounded-md border border-border bg-background/60 px-2 py-1 text-sm"
                          value={retryVal}
                          onChange={(e) => saveRetry(parseInt(e.target.value || '0', 10)).catch((err) => console.error(err))}
                        />
                      </div>
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm text-muted-foreground">Max retry interval (s)</span>
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          min={0}
                          max={3600}
                          className="w-24 rounded-md border border-border bg-background/60 px-2 py-1 text-sm"
                          value={maxRetryVal}
                          onChange={(e) => saveMaxRetry(parseInt(e.target.value || '0', 10)).catch((err) => console.error(err))}
                        />
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                  <CardHeader className="pb-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="text-lg">Accounts</CardTitle>
                        <CardDescription>Loaded auth files</CardDescription>
                      </div>
                      {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => loadAuthFiles().catch((e) => console.error(e))}>
                        Refresh
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => resetCooldown().catch((e) => console.error(e))}>
                        Reset all cooldowns
                      </Button>
                    </div>
                    <div className="max-h-56 overflow-auto rounded-md border border-border/50">
                      <table className="w-full text-xs">
                        <thead className="bg-muted/40">
                          <tr>
                            <th className="px-2 py-2 text-left">Provider</th>
                            <th className="px-2 py-2 text-left">Email/Label</th>
                            <th className="px-2 py-2 text-left">Status</th>
                            <th className="px-2 py-2 text-left">Action</th>
                          </tr>
                        </thead>
                        <tbody>
                          {authFiles.length === 0 && (
                            <tr>
                              <td className="px-2 py-2 text-muted-foreground" colSpan={4}>
                                No auth files loaded.
                              </td>
                            </tr>
                          )}
                          {authFiles.map((f) => (
                            <tr key={f.id} className="border-t border-border/40">
                              <td className="px-2 py-2 font-mono">{f.provider || f.type || '-'}</td>
                              <td className="px-2 py-2">{f.email || f.label || '-'}</td>
                              <td className="px-2 py-2">{f.status || '-'}</td>
                              <td className="px-2 py-2">
                                <Button size="sm" variant="ghost" onClick={() => resetCooldown(f.id).catch((e) => console.error(e))}>
                                  Reset
                                </Button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </CardContent>
                </Card>
              </div>
            )}
          </div>
        )

      case 'routing':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <>
                <ModelMappings />
                <ConfigEditor />
              </>
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access routing settings.
              </div>
            )}
          </div>
        )

      case 'memory':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <>
                <MemoryManager />
                <SemanticMemory />
              </>
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access memory settings.
              </div>
            )}
          </div>
        )

      case 'logs':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <LogsViewer />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access logs.
              </div>
            )}
          </div>
        )

      case 'requests':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <RequestMonitor />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access request monitor.
              </div>
            )}
          </div>
        )

      case 'analytics':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <UsageStats />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access analytics.
              </div>
            )}
          </div>
        )

      default:
        return null
    }
  }

  return (
    <div
      className="text-foreground transition-colors duration-500"
      style={{
        display: 'grid',
        gridTemplateRows: '64px 1fr 32px',
        gridTemplateColumns: '64px 1fr',
        height: '100vh',
        background: 'var(--bg-void)',
      }}
    >
      {/* Header - spans full width */}
      <div style={{ gridColumn: '1 / -1' }}>
        <Header isRunning={isRunning} port={status?.port} />
      </div>

      {/* Icon Rail - left side */}
      <div style={{ gridRow: 2 }}>
        <IconRail
          items={navigationItems}
          activeId={activeView}
          onSelect={(id) => setActiveView(id as ViewId)}
        />
      </div>

      {/* Main Content - fills remaining space */}
      <main
        key={activeView}
        className="animate-fade-in-up"
        style={{
          gridRow: 2,
          overflow: 'auto',
          padding: '24px',
        }}
      >
        {renderViewContent()}
      </main>

      {/* Status Bar - spans full width */}
      <div style={{ gridColumn: '1 / -1' }}>
        <StatusBar />
      </div>

      <Toaster position="bottom-center" theme="dark" />
    </div>
  )
}

export default function App() {
  return (
    <TooltipProvider>
      <ProxyProvider>
        <DashboardContent />
      </ProxyProvider>
    </TooltipProvider>
  )
}
