import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Play,
  Square,
  RotateCcw,
  FolderOpen,
  FileText,
  Copy,
  Activity,
  Settings,
  Shield,
  Globe,
  AlertCircle,
} from 'lucide-react'
import { useProxyContext } from '@/hooks/useProxyContext'

export function EngineControl() {
  const { status, loading, handleAction, showToast } = useProxyContext()
  const [privateOAuth, setPrivateOAuth] = useState(false)

  const isRunning = status?.running ?? false

  useEffect(() => {
    ;(async () => {
      try {
        if (window.pp_get_oauth_private) {
          const priv = await window.pp_get_oauth_private()
          setPrivateOAuth(priv)
        }
      } catch (e) {
        console.error('OAuth private error:', e)
      }
    })()
  }, [])

  const handlePrivateOAuthChange = async (checked: boolean) => {
    try {
      if (window.pp_set_oauth_private) {
        await window.pp_set_oauth_private(checked)
        setPrivateOAuth(checked)
        showToast('Preference saved', 'success')
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
      <CardHeader className="pb-4">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-primary/10 flex items-center justify-center">
            <Activity className="h-5 w-5 text-primary" />
          </div>
          <div>
            <CardTitle className="text-lg">Engine Control</CardTitle>
            <CardDescription>Start, stop or restart the proxy</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {/* Control Buttons */}
        <div className="flex gap-3">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                className={`flex-1 gap-2 transition-all duration-200 font-semibold shadow-md ${
                  isRunning
                    ? 'opacity-50 cursor-not-allowed'
                    : 'hover:scale-[1.02] active:scale-[0.98]'
                }`}
                variant={isRunning ? 'secondary' : 'default'}
                disabled={isRunning || loading === 'start'}
                onClick={() => handleAction(window.pp_start, 'start', 'Proxy started')}
              >
                <Play className="h-4 w-4 fill-current" />
                Start
              </Button>
            </TooltipTrigger>
            <TooltipContent>Start the proxy server</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="destructive"
                className={`flex-1 gap-2 transition-all duration-200 shadow-md ${
                  !isRunning
                    ? 'opacity-50 cursor-not-allowed'
                    : 'hover:bg-destructive/90 hover:scale-[1.02]'
                }`}
                disabled={!isRunning || loading === 'stop'}
                onClick={() => handleAction(window.pp_stop, 'stop', 'Proxy stopped')}
              >
                <Square className="h-4 w-4 fill-current" />
                Stop
              </Button>
            </TooltipTrigger>
            <TooltipContent>Stop the proxy server</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                className={`flex-1 gap-2 transition-all duration-200 ${
                  !isRunning
                    ? 'opacity-50 cursor-not-allowed'
                    : 'hover:bg-secondary hover:text-secondary-foreground'
                }`}
                disabled={!isRunning || loading === 'restart'}
                onClick={() => handleAction(window.pp_restart, 'restart', 'Proxy restarted')}
              >
                <RotateCcw className="h-4 w-4" />
                Restart
              </Button>
            </TooltipTrigger>
            <TooltipContent>Restart the proxy server</TooltipContent>
          </Tooltip>
        </div>

        {/* Status Info */}
        <div className="rounded-xl bg-muted/50 border border-border/50 p-4 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground flex items-center gap-2">
              <Globe className="h-4 w-4" /> Base URL
            </span>
            <code className="text-sm font-mono text-primary bg-background/50 px-2 py-1 rounded border border-border/50">
              {status?.base_url || '—'}
            </code>
          </div>
          <Separator className="bg-border/50" />
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Port</span>
            <span className="text-sm font-mono">{status?.port || '—'}</span>
          </div>
          {status?.last_error && (
            <>
              <Separator className="bg-border/50" />
              <div className="flex items-start gap-2 text-destructive">
                <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
                <span className="text-sm break-all font-medium">{status.last_error}</span>
              </div>
            </>
          )}
        </div>

        {/* Quick Actions */}
        <div className="flex flex-wrap gap-2">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="gap-2 text-muted-foreground hover:text-foreground"
                onClick={() => handleAction(window.pp_open_logs, 'logs', 'Opened logs')}
              >
                <FileText className="h-4 w-4" /> Logs
              </Button>
            </TooltipTrigger>
            <TooltipContent>View log files</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="gap-2 text-muted-foreground hover:text-foreground"
                onClick={() => handleAction(window.pp_open_auth_folder, 'auth', 'Opened auth folder')}
              >
                <FolderOpen className="h-4 w-4" /> Auth Folder
              </Button>
            </TooltipTrigger>
            <TooltipContent>Open authentication files folder</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="gap-2 text-muted-foreground hover:text-foreground"
                onClick={() => handleAction(window.pp_copy_diagnostics, 'diag-copy', 'Copied to clipboard')}
              >
                <Copy className="h-4 w-4" /> Copy Diagnostics
              </Button>
            </TooltipTrigger>
            <TooltipContent>Copy diagnostics to clipboard</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="gap-2 text-muted-foreground hover:text-foreground"
                disabled={!isRunning}
                onClick={() => handleAction(window.pp_open_diagnostics, 'diag-open', 'Opened diagnostics')}
              >
                <Activity className="h-4 w-4" /> Diagnostics
              </Button>
            </TooltipTrigger>
            <TooltipContent>Open diagnostics panel</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="gap-2 text-muted-foreground hover:text-foreground"
                disabled={!isRunning}
                onClick={() => handleAction(window.pp_open_legacy_ui, 'legacy', 'Opened advanced UI')}
              >
                <Settings className="h-4 w-4" /> Advanced
              </Button>
            </TooltipTrigger>
            <TooltipContent>Open advanced management UI</TooltipContent>
          </Tooltip>
        </div>

        {/* Private OAuth Toggle */}
        <div className="flex items-center justify-between rounded-lg bg-muted/30 border border-border/50 p-3">
          <div className="flex items-center gap-3">
            <Shield className="h-5 w-5 text-muted-foreground" />
            <Label htmlFor="private-oauth" className="text-sm cursor-pointer">
              Open OAuth in private window
            </Label>
          </div>
          <Switch
            id="private-oauth"
            checked={privateOAuth}
            onCheckedChange={handlePrivateOAuthChange}
          />
        </div>
      </CardContent>
    </Card>
  )
}
