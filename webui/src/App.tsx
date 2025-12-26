import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ThemeToggle } from '@/components/ui/theme-toggle'
import { Zap, Check, AlertCircle } from 'lucide-react'
import { ProxyProvider, useProxyContext } from '@/hooks/useProxyContext'
import {
  EngineControl,
  ProviderLogins,
  Integrations,
  ModelMappings,
  MemoryManager,
  SemanticMemory,
  LogsViewer,
} from '@/components/dashboard'

function DashboardContent() {
  const { status, isDesktop, mgmtKey, mgmtFetch, showToast, toast } = useProxyContext()
  const [mgmtError, setMgmtError] = useState<string | null>(null)
  const [mgmtConfig, setMgmtConfig] = useState<any>(null)
  const [authFiles, setAuthFiles] = useState<any[]>([])

  const isRunning = status?.running ?? false

  const loadMgmtConfig = async () => {
    const cfg = await mgmtFetch('/v0/management/config')
    setMgmtConfig(cfg)
  }

  const toggleDebug = async () => {
    const cur = await mgmtFetch('/v0/management/debug')
    await mgmtFetch('/v0/management/debug', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ value: !cur.value }),
    })
    await loadMgmtConfig()
  }

  const saveRetry = async (value: number) => {
    await mgmtFetch('/v0/management/request-retry', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ value }),
    })
    await loadMgmtConfig()
  }

  const saveMaxRetry = async (value: number) => {
    await mgmtFetch('/v0/management/max-retry-interval', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ value }),
    })
    await loadMgmtConfig()
  }

  const loadAuthFiles = async () => {
    const res = await mgmtFetch('/v0/management/auth-files')
    setAuthFiles(res.files || [])
  }

  const resetCooldown = async (authId?: string) => {
    await mgmtFetch('/v0/management/auth/reset-cooldown', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(authId ? { auth_id: authId } : {}),
    })
    await loadAuthFiles()
  }

  useEffect(() => {
    if (!mgmtKey) return
    setMgmtError(null)
    ;(async () => {
      try {
        await loadMgmtConfig()
        await loadAuthFiles()
      } catch (e) {
        setMgmtError(e instanceof Error ? e.message : String(e))
      }
    })()
  }, [mgmtKey])

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

  return (
    <div className="min-h-screen bg-background text-foreground relative overflow-hidden transition-colors duration-500">
      {/* Animated background effects */}
      <div className="fixed inset-0 pointer-events-none">
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-full h-full bg-[radial-gradient(ellipse_at_top,_var(--primary)_0%,_transparent_20%)] opacity-20" />
        <div className="absolute -top-40 -right-40 w-96 h-96 bg-primary/20 rounded-full blur-3xl animate-pulse" />
        <div className="absolute top-1/2 -left-40 w-80 h-80 bg-secondary/20 rounded-full blur-3xl animate-pulse delay-1000" />
      </div>

      <div className="relative mx-auto max-w-5xl p-6 space-y-6">
        {/* Header */}
        <header className="flex items-center justify-between py-4">
          <div className="flex items-center gap-4">
            <div className="relative">
              <div className="h-14 w-14 rounded-2xl bg-gradient-to-br from-primary to-ring shadow-lg shadow-primary/25 flex items-center justify-center">
                <Zap className="h-7 w-7 text-primary-foreground" />
              </div>
              {isRunning && (
                <span className="absolute -top-1 -right-1 h-4 w-4 rounded-full bg-green-500 border-2 border-background animate-pulse" />
              )}
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight">ProxyPilot</h1>
              <p className="text-sm text-muted-foreground">Local AI proxy controller</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <ThemeToggle />
            <Badge
              variant={isRunning ? 'default' : 'secondary'}
              className={`gap-2 px-4 py-2 text-sm font-medium transition-all duration-300 ${
                isRunning
                  ? 'bg-green-500/15 text-green-500 border-green-500/20 hover:bg-green-500/25'
                  : 'bg-muted text-muted-foreground border-border'
              }`}
            >
              <span className={`h-2 w-2 rounded-full ${isRunning ? 'bg-green-500 animate-pulse' : 'bg-amber-500'}`} />
              {isRunning ? `Running on :${status?.port}` : 'Stopped'}
            </Badge>
          </div>
        </header>

        {/* Tabs Navigation */}
        <Tabs defaultValue="engine" className="space-y-6">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="engine">Engine</TabsTrigger>
            <TabsTrigger value="auth">Auth</TabsTrigger>
            <TabsTrigger value="routing">Routing</TabsTrigger>
            <TabsTrigger value="memory">Memory</TabsTrigger>
            <TabsTrigger value="logs">Logs</TabsTrigger>
          </TabsList>

          {/* Engine Tab */}
          <TabsContent value="engine" className="space-y-6">
            <div className="grid gap-6 lg:grid-cols-2">
              <EngineControl />
              <ProviderLogins />
            </div>
            <Integrations />
          </TabsContent>

          {/* Auth Tab */}
          <TabsContent value="auth" className="space-y-6">
            {!isDesktop && !mgmtKey && (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key missing. Start ProxyPilot from the tray app to inject a local key,
                or set `MANAGEMENT_PASSWORD` before loading this page.
              </div>
            )}

            {mgmtError && (
              <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
                Management error: {mgmtError}
              </div>
            )}

            {mgmtKey && (
              <div className="grid gap-6 lg:grid-cols-2">
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Quick Config</CardTitle>
                    <CardDescription>Management API settings</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-muted-foreground">Debug</span>
                      <div className="flex items-center gap-2">
                        <Badge variant={debugOn ? 'default' : 'secondary'}>
                          {debugOn ? 'On' : 'Off'}
                        </Badge>
                        <Button size="sm" variant="outline" onClick={() => toggleDebug().catch((e) => showToast(String(e), 'error'))}>
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
                          onChange={(e) => saveRetry(parseInt(e.target.value || '0', 10)).catch((err) => showToast(String(err), 'error'))}
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
                          onChange={(e) => saveMaxRetry(parseInt(e.target.value || '0', 10)).catch((err) => showToast(String(err), 'error'))}
                        />
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Accounts</CardTitle>
                    <CardDescription>Loaded auth files</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => loadAuthFiles().catch((e) => showToast(String(e), 'error'))}>
                        Refresh
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => resetCooldown().catch((e) => showToast(String(e), 'error'))}>
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
                                <Button size="sm" variant="ghost" onClick={() => resetCooldown(f.id).catch((e) => showToast(String(e), 'error'))}>
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
          </TabsContent>

          {/* Routing Tab */}
          <TabsContent value="routing" className="space-y-6">
            {mgmtKey ? (
              <ModelMappings />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access routing settings.
              </div>
            )}
          </TabsContent>

          {/* Memory Tab */}
          <TabsContent value="memory" className="space-y-6">
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
          </TabsContent>

          {/* Logs Tab */}
          <TabsContent value="logs" className="space-y-6">
            {mgmtKey ? (
              <LogsViewer />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access logs.
              </div>
            )}
          </TabsContent>
        </Tabs>

        {/* Footer */}
        <footer className="text-center text-xs text-muted-foreground pt-4 opacity-60 hover:opacity-100 transition-opacity">
          ProxyPilot - Local AI Proxy Controller
        </footer>
      </div>

      {/* Toast Notification */}
      {toast && (
        <div
          className={`fixed bottom-6 left-1/2 -translate-x-1/2 flex items-center gap-2 rounded-lg px-4 py-3 text-sm shadow-lg backdrop-blur-md animate-in slide-in-from-bottom-4 fade-in duration-300 ${
            toast.type === 'success'
              ? 'bg-green-500/10 text-green-500 border border-green-500/20'
              : 'bg-destructive/10 text-destructive border border-destructive/20'
          }`}
        >
          {toast.type === 'success' ? (
            <Check className="h-4 w-4" />
          ) : (
            <AlertCircle className="h-4 w-4" />
          )}
          {toast.message}
        </div>
      )}
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
