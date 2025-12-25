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
    pp_get_management_key?: () => Promise<string>;
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
  const [mgmtKey, setMgmtKey] = useState<string | null>(null);
  const [mgmtError, setMgmtError] = useState<string | null>(null);
  const [mgmtConfig, setMgmtConfig] = useState<any>(null);
  const [authFiles, setAuthFiles] = useState<any[]>([]);
  const [routingModel, setRoutingModel] = useState('');
  const [routingJSON, setRoutingJSON] = useState('');
  const [routingPreview, setRoutingPreview] = useState('');
  const [recentRouting, setRecentRouting] = useState<any[]>([]);
  const [diagnostics, setDiagnostics] = useState('');
  const [logText, setLogText] = useState('');
  const [semanticHealth, setSemanticHealth] = useState<any>(null);
  const [semanticNamespaces, setSemanticNamespaces] = useState<any[]>([]);
  const [semanticNamespace, setSemanticNamespace] = useState('');
  const [semanticLimit, setSemanticLimit] = useState(50);
  const [semanticItems, setSemanticItems] = useState('');
  const [memorySessions, setMemorySessions] = useState<any[]>([]);
  const [memorySession, setMemorySession] = useState('');
  const [memoryDetails, setMemoryDetails] = useState<any>(null);
  const [memoryEvents, setMemoryEvents] = useState('');
  const [memoryAnchors, setMemoryAnchors] = useState('');
  const [memoryEventsLimit, setMemoryEventsLimit] = useState(120);
  const [memoryAnchorsLimit, setMemoryAnchorsLimit] = useState(20);
  const [memoryTodo, setMemoryTodo] = useState('');
  const [memoryPinned, setMemoryPinned] = useState('');
  const [memorySummary, setMemorySummary] = useState('');
  const [memoryImportReplace, setMemoryImportReplace] = useState(false);
  const [memorySemanticEnabled, setMemorySemanticEnabled] = useState(true);
  const [memoryPrune, setMemoryPrune] = useState({
    maxAgeDays: 30,
    maxSessions: 200,
    maxBytesPerSession: 2000000,
    maxNamespaces: 200,
    maxBytesPerNamespace: 2000000,
  });

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 2500);
  };

  const isDesktop = typeof window.pp_status === 'function';

  const refreshStatus = async () => {
    try {
      if (window.pp_status) {
        const s = await window.pp_status();
        setStatus(s);
      } else {
        const res = await fetch('/healthz');
        if (!res.ok) {
          setStatus({ running: false, port: 0, base_url: location.origin, last_error: res.statusText });
          return;
        }
        const body = await res.json().catch(() => ({}));
        setStatus({
          running: true,
          port: body.port || 0,
          base_url: location.origin,
          last_error: '',
        });
      }
    } catch (e) {
      console.error('Status error:', e);
    }
  };

  const getMgmtKey = () => {
    const meta = document.querySelector('meta[name="pp-mgmt-key"]');
    return meta ? meta.getAttribute('content') : null;
  };

  const mgmtFetch = async (path: string, opts: RequestInit = {}) => {
    if (!mgmtKey) {
      throw new Error('Missing management key');
    }
    const headers = Object.assign({}, opts.headers || {}, { 'X-Management-Key': mgmtKey });
    const res = await fetch(path, { ...opts, headers });
    const ct = (res.headers.get('content-type') || '').toLowerCase();
    const body = ct.includes('application/json') ? await res.json() : await res.text();
    if (!res.ok) {
      const msg = typeof body === 'string' ? body : body?.error ? body.error : JSON.stringify(body);
      throw new Error(`${res.status} ${res.statusText}: ${msg}`);
    }
    return body;
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
        if (window.pp_get_management_key) {
          const key = await window.pp_get_management_key();
          setMgmtKey(key);
        } else if (!isDesktop) {
          const key = getMgmtKey();
          setMgmtKey(key);
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

  const loadMgmtConfig = async () => {
    const cfg = await mgmtFetch('/v0/management/config');
    setMgmtConfig(cfg);
  };

  const toggleDebug = async () => {
    const cur = await mgmtFetch('/v0/management/debug');
    await mgmtFetch('/v0/management/debug', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ value: !cur.value }),
    });
    await loadMgmtConfig();
  };

  const saveRetry = async (value: number) => {
    await mgmtFetch('/v0/management/request-retry', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ value }),
    });
    await loadMgmtConfig();
  };

  const saveMaxRetry = async (value: number) => {
    await mgmtFetch('/v0/management/max-retry-interval', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ value }),
    });
    await loadMgmtConfig();
  };

  const loadAuthFiles = async () => {
    const res = await mgmtFetch('/v0/management/auth-files');
    setAuthFiles(res.files || []);
  };

  const resetCooldown = async (authId?: string) => {
    await mgmtFetch('/v0/management/auth/reset-cooldown', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(authId ? { auth_id: authId } : {}),
    });
    await loadAuthFiles();
  };

  const loadRoutingPreview = async () => {
    if (!routingModel.trim()) {
      setRoutingPreview('Enter a model name to preview routing.');
      return;
    }
    const res = await mgmtFetch(`/v0/management/routing/preview?model=${encodeURIComponent(routingModel.trim())}`);
    setRoutingPreview(JSON.stringify(res, null, 2));
  };

  const loadRoutingPreviewFromJSON = async () => {
    if (!routingJSON.trim()) {
      setRoutingPreview('Paste a full request JSON to preview routing.');
      return;
    }
    const res = await mgmtFetch('/v0/management/routing/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: routingJSON,
    });
    setRoutingPreview(JSON.stringify(res, null, 2));
  };

  const loadDiagnostics = async () => {
    const res = await mgmtFetch('/v0/management/proxypilot/diagnostics?lines=120');
    setDiagnostics(res.text || '');
  };

  const copyDiagnostics = async () => {
    if (!diagnostics) return;
    await navigator.clipboard.writeText(diagnostics);
    showToast('Copied diagnostics', 'success');
  };

  const tailLogs = async (kind: 'stdout' | 'stderr') => {
    const res = await mgmtFetch(`/v0/management/proxypilot/logs/tail?file=${kind}&lines=200`);
    setLogText((res.lines || []).join('\n'));
  };

  const loadRecentRouting = async () => {
    const res = await mgmtFetch('/v0/management/routing/recent');
    setRecentRouting(res.entries || []);
  };

  const loadSemanticHealth = async () => {
    const res = await mgmtFetch('/v0/management/semantic/health');
    setSemanticHealth(res);
  };

  const loadSemanticNamespaces = async () => {
    const res = await mgmtFetch('/v0/management/semantic/namespaces');
    setSemanticNamespaces(res.namespaces || []);
    if (!semanticNamespace && res.namespaces && res.namespaces.length > 0) {
      setSemanticNamespace(res.namespaces[0].key);
    }
  };

  const loadSemanticItems = async () => {
    if (!semanticNamespace) {
      setSemanticItems('Select a namespace.');
      return;
    }
    const res = await mgmtFetch(`/v0/management/semantic/items?namespace=${encodeURIComponent(semanticNamespace)}&limit=${encodeURIComponent(semanticLimit)}`);
    const items = res.items || [];
    if (!Array.isArray(items) || items.length === 0) {
      setSemanticItems('No items.');
      return;
    }
    const lines = items.map((it: any) => {
      const ts = it.ts || '';
      const src = it.source || '';
      const role = it.role || '';
      const text = (it.text || '').toString();
      return `[${ts}][${src}][${role}] ${text}`;
    });
    setSemanticItems(lines.join('\n\n'));
  };

  const loadMemorySessions = async () => {
    const res = await mgmtFetch('/v0/management/memory/sessions?limit=200');
    const sessions = res.sessions || [];
    setMemorySessions(sessions);
    if (!memorySession && sessions.length > 0) {
      setMemorySession(sessions[0].key);
    }
  };

  const loadMemorySessionDetails = async () => {
    if (!memorySession) {
      setMemoryDetails(null);
      return;
    }
    const res = await mgmtFetch(`/v0/management/memory/session?session=${encodeURIComponent(memorySession)}`);
    const session = res.session || null;
    setMemoryDetails(session);
    if (session) {
      setMemorySummary(session.summary || '');
      setMemoryPinned(session.pinned || '');
      setMemoryTodo(session.todo || '');
      setMemorySemanticEnabled(!session.semantic_disabled);
    }
  };

  const loadMemoryEvents = async () => {
    if (!memorySession) {
      setMemoryEvents('Select a session.');
      return;
    }
    const res = await mgmtFetch(`/v0/management/memory/events?session=${encodeURIComponent(memorySession)}&limit=${encodeURIComponent(memoryEventsLimit)}`);
    const events = res.events || [];
    if (!Array.isArray(events) || events.length === 0) {
      setMemoryEvents('No events.');
      return;
    }
    const lines = events.map((e: any) => {
      const ts = e.ts || '';
      const kind = e.kind || '';
      const role = e.role || '';
      const text = (e.text || '').toString();
      return `[${ts}][${kind}][${role}] ${text}`;
    });
    setMemoryEvents(lines.join('\n\n'));
  };

  const loadMemoryAnchors = async () => {
    if (!memorySession) {
      setMemoryAnchors('Select a session.');
      return;
    }
    const res = await mgmtFetch(`/v0/management/memory/anchors?session=${encodeURIComponent(memorySession)}&limit=${encodeURIComponent(memoryAnchorsLimit)}`);
    const anchors = res.anchors || [];
    if (!Array.isArray(anchors) || anchors.length === 0) {
      setMemoryAnchors('No anchors.');
      return;
    }
    const lines = anchors.map((a: any) => {
      const ts = a.ts || '';
      const summary = (a.summary || '').toString();
      return `[${ts}]\n${summary}`;
    });
    setMemoryAnchors(lines.join('\n\n---\n\n'));
  };

  const saveMemoryTodo = async () => {
    if (!memorySession) return;
    await mgmtFetch('/v0/management/memory/todo', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session: memorySession, value: memoryTodo }),
    });
    await loadMemorySessionDetails();
    showToast('Saved TODO', 'success');
  };

  const saveMemoryPinned = async () => {
    if (!memorySession) return;
    await mgmtFetch('/v0/management/memory/pinned', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session: memorySession, value: memoryPinned }),
    });
    await loadMemorySessionDetails();
    showToast('Saved pinned context', 'success');
  };

  const saveMemorySummary = async () => {
    if (!memorySession) return;
    await mgmtFetch('/v0/management/memory/summary', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session: memorySession, value: memorySummary }),
    });
    await loadMemorySessionDetails();
    showToast('Saved anchor summary', 'success');
  };

  const toggleMemorySemantic = async (enabled: boolean) => {
    if (!memorySession) return;
    await mgmtFetch('/v0/management/memory/semantic', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session: memorySession, enabled }),
    });
    setMemorySemanticEnabled(enabled);
    await loadMemorySessionDetails();
    showToast(`Semantic ${enabled ? 'enabled' : 'disabled'}`, 'success');
  };

  const deleteMemorySession = async () => {
    if (!memorySession) return;
    await mgmtFetch(`/v0/management/memory/session?session=${encodeURIComponent(memorySession)}`, {
      method: 'DELETE',
    });
    setMemorySession('');
    setMemoryDetails(null);
    setMemoryEvents('');
    setMemoryAnchors('');
    await loadMemorySessions();
    showToast('Deleted session', 'success');
  };

  const exportMemorySession = async () => {
    if (!memorySession || !mgmtKey) return;
    const res = await fetch(`/v0/management/memory/export?session=${encodeURIComponent(memorySession)}`, {
      headers: { 'X-Management-Key': mgmtKey },
    });
    if (!res.ok) {
      const msg = await res.text();
      throw new Error(msg);
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `proxypilot-session-${memorySession}.zip`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const exportAllMemory = async () => {
    if (!mgmtKey) return;
    const res = await fetch('/v0/management/memory/export-all', {
      headers: { 'X-Management-Key': mgmtKey },
    });
    if (!res.ok) {
      const msg = await res.text();
      throw new Error(msg);
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'proxypilot-memory-all.zip';
    a.click();
    URL.revokeObjectURL(url);
  };

  const deleteAllMemory = async () => {
    if (!mgmtKey) return;
    if (!window.confirm('Delete all memory data? This cannot be undone.')) return;
    await mgmtFetch('/v0/management/memory/delete-all?confirm=true', { method: 'POST' });
    setMemorySession('');
    setMemoryDetails(null);
    setMemoryEvents('');
    setMemoryAnchors('');
    await loadMemorySessions();
    showToast('Deleted all memory', 'success');
  };

  const importMemorySession = async (file: File | null) => {
    if (!file || !memorySession || !mgmtKey) return;
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`/v0/management/memory/import?session=${encodeURIComponent(memorySession)}&replace=${memoryImportReplace ? 'true' : 'false'}`, {
      method: 'POST',
      headers: { 'X-Management-Key': mgmtKey },
      body: form,
    });
    if (!res.ok) {
      const msg = await res.text();
      throw new Error(msg);
    }
    await loadMemorySessionDetails();
    showToast('Imported session', 'success');
  };

  const pruneMemory = async () => {
    await mgmtFetch('/v0/management/memory/prune', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        max_age_days: memoryPrune.maxAgeDays,
        max_sessions: memoryPrune.maxSessions,
        max_bytes_per_session: memoryPrune.maxBytesPerSession,
        max_namespaces: memoryPrune.maxNamespaces,
        max_bytes_per_namespace: memoryPrune.maxBytesPerNamespace,
      }),
    });
    await loadMemorySessions();
    showToast('Prune completed', 'success');
  };

  useEffect(() => {
    if (!mgmtKey) return;
    setMgmtError(null);
    (async () => {
      try {
        await loadMgmtConfig();
        await loadAuthFiles();
        await loadRecentRouting();
        await loadSemanticHealth();
        await loadSemanticNamespaces();
        await loadMemorySessions();
        await tailLogs('stdout');
      } catch (e) {
        setMgmtError(e instanceof Error ? e.message : String(e));
      }
    })();
  }, [mgmtKey]);

  const isRunning = status?.running ?? false;
  const debugOn = !!mgmtConfig?.debug;
  const retryVal =
    mgmtConfig?.['request-retry'] ??
    mgmtConfig?.request_retry ??
    mgmtConfig?.requestRetry ??
    0;
  const maxRetryVal =
    mgmtConfig?.['max-retry-interval'] ??
    mgmtConfig?.max_retry_interval ??
    mgmtConfig?.maxRetryInterval ??
    0;

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
            <>
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

              <div className="grid gap-6 lg:grid-cols-3">
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl lg:col-span-1">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Routing Preview</CardTitle>
                    <CardDescription>How a model will be routed</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <input
                      className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                      placeholder="model name (auto, gpt-5.2, provider://model)"
                      value={routingModel}
                      onChange={(e) => setRoutingModel(e.target.value)}
                    />
                    <Button size="sm" variant="outline" onClick={() => loadRoutingPreview().catch((e) => showToast(String(e), 'error'))}>
                      Preview
                    </Button>
                    <textarea
                      className="h-24 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                      placeholder={'{ "model": "gpt-5.2" }'}
                      value={routingJSON}
                      onChange={(e) => setRoutingJSON(e.target.value)}
                    />
                    <Button size="sm" variant="outline" onClick={() => loadRoutingPreviewFromJSON().catch((e) => showToast(String(e), 'error'))}>
                      Preview from JSON
                    </Button>
                    <pre className="max-h-48 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                      {routingPreview || 'No preview yet.'}
                    </pre>
                  </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl lg:col-span-1">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Diagnostics</CardTitle>
                    <CardDescription>Engine snapshot</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => loadDiagnostics().catch((e) => showToast(String(e), 'error'))}>
                        Load
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => copyDiagnostics().catch((e) => showToast(String(e), 'error'))}>
                        Copy
                      </Button>
                    </div>
                    <pre className="max-h-48 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                      {diagnostics || 'No diagnostics loaded.'}
                    </pre>
                  </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl lg:col-span-1">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Recent Routing</CardTitle>
                    <CardDescription>Last routed requests</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <Button size="sm" variant="outline" onClick={() => loadRecentRouting().catch((e) => showToast(String(e), 'error'))}>
                      Refresh
                    </Button>
                    <div className="max-h-48 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                      {recentRouting.length === 0 && <div className="text-muted-foreground">No entries.</div>}
                      {recentRouting.map((e) => (
                        <div key={e.request_id || e.timestamp} className="border-b border-border/40 py-1">
                          <div className="font-mono">{e.path}</div>
                          <div className="text-muted-foreground">
                            {e.requested_model} → {e.resolved_model} [{e.selected_provider}]
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              </div>

              <div className="grid gap-6 lg:grid-cols-2">
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl lg:col-span-2">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Memory</CardTitle>
                    <CardDescription>Anchors, pinned context, TODOs, and session history</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" variant="outline" onClick={() => loadMemorySessions().catch((e) => showToast(String(e), 'error'))}>
                        Refresh sessions
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => loadMemorySessionDetails().catch((e) => showToast(String(e), 'error'))}>
                        Load session
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => loadMemoryEvents().catch((e) => showToast(String(e), 'error'))}>
                        Events
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => loadMemoryAnchors().catch((e) => showToast(String(e), 'error'))}>
                        Anchors
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => exportMemorySession().catch((e) => showToast(String(e), 'error'))}>
                        Export
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => exportAllMemory().catch((e) => showToast(String(e), 'error'))}>
                        Export all
                      </Button>
                      <Button size="sm" variant="destructive" onClick={() => deleteMemorySession().catch((e) => showToast(String(e), 'error'))}>
                        Delete
                      </Button>
                      <Button size="sm" variant="destructive" onClick={() => deleteAllMemory().catch((e) => showToast(String(e), 'error'))}>
                        Delete all
                      </Button>
                    </div>

                    <div className="flex flex-wrap items-center gap-2">
                      <select
                        className="min-w-[220px] rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                        value={memorySession}
                        onChange={(e) => setMemorySession(e.target.value)}
                      >
                        {memorySessions.length === 0 && <option value="">(no sessions)</option>}
                        {memorySessions.map((s) => (
                          <option key={s.key} value={s.key}>
                            {s.key}
                          </option>
                        ))}
                      </select>
                      <input
                        type="number"
                        min={10}
                        max={500}
                        className="w-24 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                        value={memoryEventsLimit}
                        onChange={(e) => setMemoryEventsLimit(parseInt(e.target.value || '120', 10))}
                      />
                      <input
                        type="number"
                        min={5}
                        max={200}
                        className="w-24 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                        value={memoryAnchorsLimit}
                        onChange={(e) => setMemoryAnchorsLimit(parseInt(e.target.value || '20', 10))}
                      />
                      <span className="text-xs text-muted-foreground">
                        {memoryDetails?.updated_at ? `Updated ${memoryDetails.updated_at}` : 'No session loaded.'}
                      </span>
                      <label className="text-xs text-muted-foreground flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={memorySemanticEnabled}
                          disabled={!memorySession}
                          onChange={(e) => toggleMemorySemantic(e.target.checked).catch((err) => showToast(String(err), 'error'))}
                        />
                        Semantic enabled
                      </label>
                    </div>

                    <div className="grid gap-4 lg:grid-cols-3">
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground">Anchor summary</div>
                        <textarea
                          className="h-40 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                          value={memorySummary}
                          onChange={(e) => setMemorySummary(e.target.value)}
                        />
                        <Button size="sm" variant="outline" onClick={() => saveMemorySummary().catch((e) => showToast(String(e), 'error'))}>
                          Save summary
                        </Button>
                      </div>
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground">Pinned context</div>
                        <textarea
                          className="h-40 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                          value={memoryPinned}
                          onChange={(e) => setMemoryPinned(e.target.value)}
                        />
                        <Button size="sm" variant="outline" onClick={() => saveMemoryPinned().catch((e) => showToast(String(e), 'error'))}>
                          Save pinned
                        </Button>
                      </div>
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground">TODO</div>
                        <textarea
                          className="h-40 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                          value={memoryTodo}
                          onChange={(e) => setMemoryTodo(e.target.value)}
                        />
                        <Button size="sm" variant="outline" onClick={() => saveMemoryTodo().catch((e) => showToast(String(e), 'error'))}>
                          Save TODO
                        </Button>
                      </div>
                    </div>

                    <div className="grid gap-4 lg:grid-cols-2">
                      <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                        {memoryEvents || 'No events loaded.'}
                      </pre>
                      <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                        {memoryAnchors || 'No anchors loaded.'}
                      </pre>
                    </div>

                    <div className="grid gap-4 lg:grid-cols-2">
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground">Import session (zip)</div>
                        <div className="flex items-center gap-2">
                          <input
                            type="file"
                            accept=".zip"
                            className="text-xs"
                            onChange={(e) => importMemorySession(e.target.files?.[0] || null).catch((err) => showToast(String(err), 'error'))}
                          />
                          <label className="text-xs text-muted-foreground flex items-center gap-2">
                            <input
                              type="checkbox"
                              checked={memoryImportReplace}
                              onChange={(e) => setMemoryImportReplace(e.target.checked)}
                            />
                            Replace
                          </label>
                        </div>
                      </div>
                      <div className="space-y-2">
                        <div className="text-xs text-muted-foreground">Prune limits</div>
                        <div className="grid grid-cols-2 gap-2">
                          <input
                            type="number"
                            min={0}
                            className="rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                            value={memoryPrune.maxAgeDays}
                            onChange={(e) => setMemoryPrune({ ...memoryPrune, maxAgeDays: parseInt(e.target.value || '0', 10) })}
                            placeholder="Max age (days)"
                          />
                          <input
                            type="number"
                            min={0}
                            className="rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                            value={memoryPrune.maxSessions}
                            onChange={(e) => setMemoryPrune({ ...memoryPrune, maxSessions: parseInt(e.target.value || '0', 10) })}
                            placeholder="Max sessions"
                          />
                          <input
                            type="number"
                            min={0}
                            className="rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                            value={memoryPrune.maxNamespaces}
                            onChange={(e) => setMemoryPrune({ ...memoryPrune, maxNamespaces: parseInt(e.target.value || '0', 10) })}
                            placeholder="Max namespaces"
                          />
                          <input
                            type="number"
                            min={0}
                            className="rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                            value={memoryPrune.maxBytesPerSession}
                            onChange={(e) => setMemoryPrune({ ...memoryPrune, maxBytesPerSession: parseInt(e.target.value || '0', 10) })}
                            placeholder="Max bytes/session"
                          />
                          <input
                            type="number"
                            min={0}
                            className="rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                            value={memoryPrune.maxBytesPerNamespace}
                            onChange={(e) => setMemoryPrune({ ...memoryPrune, maxBytesPerNamespace: parseInt(e.target.value || '0', 10) })}
                            placeholder="Max bytes/namespace"
                          />
                        </div>
                        <Button size="sm" variant="outline" onClick={() => pruneMemory().catch((e) => showToast(String(e), 'error'))}>
                          Prune memory
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </div>

              <div className="grid gap-6 lg:grid-cols-2">
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Logs (tail)</CardTitle>
                    <CardDescription>Engine stdout/stderr</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex gap-2">
                      <Button size="sm" variant="outline" onClick={() => tailLogs('stdout').catch((e) => showToast(String(e), 'error'))}>
                        Stdout
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => tailLogs('stderr').catch((e) => showToast(String(e), 'error'))}>
                        Stderr
                      </Button>
                    </div>
                    <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                      {logText || 'No logs loaded.'}
                    </pre>
                  </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                  <CardHeader className="pb-4">
                    <CardTitle className="text-lg">Semantic Memory</CardTitle>
                    <CardDescription>Ollama embeddings + per-repo namespaces</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="flex items-center gap-2">
                      <Button size="sm" variant="outline" onClick={() => loadSemanticHealth().catch((e) => showToast(String(e), 'error'))}>
                        Refresh
                      </Button>
                      <Badge variant={semanticHealth?.status === 'ok' ? 'default' : 'secondary'}>
                        {semanticHealth?.status || 'unknown'}
                      </Badge>
                      <span className="text-xs text-muted-foreground">
                        {semanticHealth?.model} {semanticHealth?.version ? `v${semanticHealth.version}` : ''}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {semanticHealth?.model_present === false ? 'model missing' : ''}
                      </span>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {semanticHealth?.latency_ms ? `latency ${semanticHealth.latency_ms}ms` : 'latency n/a'}
                      {semanticHealth?.queue
                        ? ` · queue q${semanticHealth.queue.queued} d${semanticHealth.queue.dropped} p${semanticHealth.queue.processed} f${semanticHealth.queue.failed}`
                        : ''}
                    </div>
                    <div className="flex items-center gap-2">
                      <select
                        className="min-w-[200px] rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                        value={semanticNamespace}
                        onChange={(e) => setSemanticNamespace(e.target.value)}
                      >
                        {semanticNamespaces.length === 0 && <option value="">(no namespaces)</option>}
                        {semanticNamespaces.map((n) => (
                          <option key={n.key} value={n.key}>{n.label || n.key}</option>
                        ))}
                      </select>
                      <input
                        type="number"
                        min={10}
                        max={200}
                        className="w-20 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                        value={semanticLimit}
                        onChange={(e) => setSemanticLimit(parseInt(e.target.value || '50', 10))}
                      />
                      <Button size="sm" variant="outline" onClick={() => loadSemanticItems().catch((e) => showToast(String(e), 'error'))}>
                        Load
                      </Button>
                    </div>
                    <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
                      {semanticItems || 'No items loaded.'}
                    </pre>
                  </CardContent>
                </Card>
              </div>
            </>
          )}

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
