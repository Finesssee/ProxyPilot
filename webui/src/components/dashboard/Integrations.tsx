import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Terminal, Check, X } from 'lucide-react'
import { useProxyContext } from '@/hooks/useProxyContext'

interface Integration {
  id: string
  name?: string
  detected: boolean
  binary_path?: string
  config_path?: string
}

export function Integrations() {
  const { mgmtKey, mgmtFetch, showToast } = useProxyContext()
  const [integrations, setIntegrations] = useState<Integration[]>([])
  const [loading, setLoading] = useState(false)

  const fetchIntegrations = async () => {
    if (!mgmtKey) return
    setLoading(true)
    try {
      const data = await mgmtFetch('/v0/management/integrations/status')
      setIntegrations(data.integrations || [])
    } catch (e) {
      console.error('Failed to fetch integrations:', e)
    }
    setLoading(false)
  }

  const configureIntegration = async (integrationId: string) => {
    try {
      await mgmtFetch(`/v0/management/integrations/${integrationId}/configure`, {
        method: 'POST',
      })
      await fetchIntegrations()
      showToast(`Configured ${integrationId}`, 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  useEffect(() => {
    if (mgmtKey) fetchIntegrations()
  }, [mgmtKey])

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
      <CardHeader className="pb-4">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-green-500/10 flex items-center justify-center">
            <Terminal className="h-5 w-5 text-green-500" />
          </div>
          <div className="flex-1">
            <CardTitle className="text-lg">CLI Agent Integrations</CardTitle>
            <CardDescription>Detected AI coding assistants</CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => fetchIntegrations()}
            disabled={loading}
          >
            {loading ? 'Loading...' : 'Refresh'}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {integrations.length === 0 && !loading && (
            <div className="col-span-full text-center text-sm text-muted-foreground py-4">
              No integrations detected. Click Refresh to scan for CLI agents.
            </div>
          )}
          {integrations.map((integration) => (
            <div
              key={integration.id}
              className="rounded-lg border border-border/50 bg-muted/30 p-4 space-y-3"
            >
              <div className="flex items-center justify-between">
                <span className="font-medium text-sm">{integration.name || integration.id}</span>
                {integration.detected ? (
                  <Check className="h-5 w-5 text-green-500" />
                ) : (
                  <X className="h-5 w-5 text-muted-foreground" />
                )}
              </div>
              {integration.detected && (
                <>
                  {integration.binary_path && (
                    <div className="text-xs text-muted-foreground truncate" title={integration.binary_path}>
                      <span className="font-mono">Binary:</span> {integration.binary_path}
                    </div>
                  )}
                  {integration.config_path && (
                    <div className="text-xs text-muted-foreground truncate" title={integration.config_path}>
                      <span className="font-mono">Config:</span> {integration.config_path}
                    </div>
                  )}
                </>
              )}
              <Button
                size="sm"
                variant="outline"
                className="w-full"
                disabled={!integration.detected}
                onClick={() => configureIntegration(integration.id)}
              >
                Configure
              </Button>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
