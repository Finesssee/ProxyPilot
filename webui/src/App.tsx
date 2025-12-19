import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Play,
  Square,
  RotateCcw,
  FolderOpen,
  FileText,
  Copy,
  Activity,
  Settings,
  Zap,
  Shield,
  ExternalLink,
  Check,
  AlertCircle,
  Globe,
  Key,
  Info,
} from 'lucide-react'

declare global {
  interface Window {
    pp_status?: () => Promise<{ running: boolean; port: number; base_url: string; last_error: string }>;
    pp_start?: () => Promise<void>;
    pp_stop?: () => Promise<void>;
    pp_restart?: () => Promise<void>;
    pp_open_logs?: () => Promise<void>;
    pp_open_auth_folder?: () => Promise<void>;
    pp_open_legacy_ui?: () => Promise<void>;
    pp_open_diagnostics?: () => Promise<void>;
    pp_get_oauth_private?: () => Promise<boolean>;
    pp_set_oauth_private?: (enabled: boolean) => Promise<void>;
    pp_oauth?: (provider: string) => Promise<void>;
    pp_copy_diagnostics?: () => Promise<void>;
  }
}

interface ProxyStatus {
  running: boolean;
  port: number;
  base_url: string;
  last_error: string;
}

const providers = [
  { id: 'antigravity', name: 'Antigravity', color: 'from-purple-500 to-indigo-600' },
  { id: 'gemini-cli', name: 'Gemini CLI', color: 'from-blue-500 to-cyan-500' },
  { id: 'codex', name: 'Codex', color: 'from-green-500 to-emerald-600' },
  { id: 'claude', name: 'Claude', color: 'from-orange-500 to-amber-500' },
  { id: 'qwen', name: 'Qwen', color: 'from-pink-500 to-rose-500' },
  { id: 'iflow', name: 'iFlow', color: 'from-teal-500 to-cyan-600' },
]

export default function App() {
  const [status, setStatus] = useState<ProxyStatus | null>(null);
  const [privateOAuth, setPrivateOAuth] = useState(false);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);
  const [loading, setLoading] = useState<string | null>(null);

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 2500);
  };

  const refreshStatus = async () => {
    try {
      if (window.pp_status) {
        const s = await window.pp_status();
        setStatus(s);
      }
    } catch (e) {
      console.error('Status error:', e);
    }
  };

  useEffect(() => {
    refreshStatus();
    const interval = setInterval(refreshStatus, 1200);

    (async () => {
      try {
        if (window.pp_get_oauth_private) {
          const priv = await window.pp_get_oauth_private();
          setPrivateOAuth(priv);
        }
      } catch (e) {
        console.error('OAuth private error:', e);
      }
    })();

    return () => clearInterval(interval);
  }, []);

  const handleAction = async (action: (() => Promise<void>) | undefined, actionId: string, successMsg: string) => {
    if (!action) return;
    setLoading(actionId);
    try {
      await action();
      showToast(successMsg, 'success');
      await refreshStatus();
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error');
    } finally {
      setLoading(null);
    }
  };

  const handleOAuth = async (provider: string) => {
    setLoading(`oauth-${provider}`);
    try {
      if (window.pp_oauth) {
        await window.pp_oauth(provider);
        showToast(`Opening ${provider} login...`, 'success');
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error');
    } finally {
      setLoading(null);
    }
  };

  const handlePrivateOAuthChange = async (checked: boolean) => {
    try {
      if (window.pp_set_oauth_private) {
        await window.pp_set_oauth_private(checked);
        setPrivateOAuth(checked);
        showToast('Preference saved', 'success');
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error');
    }
  };

  const isRunning = status?.running ?? false;

  return (
    <TooltipProvider>
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
                <h1 className="text-2xl font-bold tracking-tight">
                  ProxyPilot
                </h1>
                <p className="text-sm text-muted-foreground">Local AI proxy controller</p>
              </div>
            </div>
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
          </header>

          {/* Main Grid */}
          <div className="grid gap-6 lg:grid-cols-2">
            {/* Engine Control Card */}
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

            {/* Provider Logins Card */}
            <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
              <CardHeader className="pb-4">
                <div className="flex items-center gap-3">
                  <div className="h-10 w-10 rounded-xl bg-purple-500/10 flex items-center justify-center">
                    <Key className="h-5 w-5 text-purple-500" />
                  </div>
                  <div>
                    <CardTitle className="text-lg">Provider Logins</CardTitle>
                    <CardDescription>Authenticate with AI providers</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-3">
                  {providers.map((provider) => (
                    <Button
                      key={provider.id}
                      variant="outline"
                      className={`relative overflow-hidden transition-all duration-300 hover:border-primary/50 group h-12 bg-background/50`}
                      onClick={() => handleOAuth(provider.id)}
                      disabled={loading === `oauth-${provider.id}`}
                    >
                      <div className={`absolute inset-0 bg-gradient-to-r ${provider.color} opacity-0 group-hover:opacity-10 transition-opacity duration-300`} />
                      <span className="relative flex items-center gap-2 font-medium">
                        <ExternalLink className="h-4 w-4 opacity-50 group-hover:opacity-100 transition-opacity" />
                        {provider.name}
                      </span>
                    </Button>
                  ))}
                </div>

                <div className="flex items-start gap-3 rounded-lg bg-blue-500/5 border border-blue-500/10 p-4">
                  <Info className="h-5 w-5 text-blue-500 mt-0.5 flex-shrink-0" />
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    Click a provider to open their login flow in your browser. Authentication tokens will be saved locally for ProxyPilot to use.
                  </p>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Footer */}
          <footer className="text-center text-xs text-muted-foreground pt-4 opacity-60 hover:opacity-100 transition-opacity">
            ProxyPilot  Local AI Proxy Controller
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
    </TooltipProvider>
  );
}