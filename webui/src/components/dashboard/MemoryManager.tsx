import { useState, useEffect, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  Database,
  Download,
  Upload,
  Trash2,
  RefreshCw,
  Save,
  FileText,
  Anchor,
  ListTodo,
  Pin,
  Scissors,
  Brain,
} from 'lucide-react'
import { useProxyContext } from '@/hooks/useProxyContext'

interface MemorySession {
  key: string;
  updated_at?: string;
  summary?: string;
  pinned?: string;
  todo?: string;
  semantic_disabled?: boolean;
}

interface PruneConfig {
  maxAgeDays: number;
  maxSessions: number;
  maxBytesPerSession: number;
  maxNamespaces: number;
  maxBytesPerNamespace: number;
}

const downloadBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
};

export function MemoryManager() {
  const { mgmtKey, mgmtFetch, showToast } = useProxyContext()

  // Session state
  const [memorySessions, setMemorySessions] = useState<MemorySession[]>([])
  const [memorySession, setMemorySession] = useState('')
  const [memoryDetails, setMemoryDetails] = useState<MemorySession | null>(null)

  // Session details
  const [memorySummary, setMemorySummary] = useState('')
  const [memoryPinned, setMemoryPinned] = useState('')
  const [memoryTodo, setMemoryTodo] = useState('')
  const [memorySemanticEnabled, setMemorySemanticEnabled] = useState(true)

  // Events and anchors
  const [memoryEvents, setMemoryEvents] = useState('')
  const [memoryAnchors, setMemoryAnchors] = useState('')
  const [memoryEventsLimit, setMemoryEventsLimit] = useState(120)
  const [memoryAnchorsLimit, setMemoryAnchorsLimit] = useState(20)

  // Import
  const [memoryImportReplace, setMemoryImportReplace] = useState(false)

  // Prune configuration
  const [memoryPrune, setMemoryPrune] = useState<PruneConfig>({
    maxAgeDays: 30,
    maxSessions: 200,
    maxBytesPerSession: 2000000,
    maxNamespaces: 200,
    maxBytesPerNamespace: 2000000,
  })

  // Load sessions list
  const loadMemorySessions = useCallback(async () => {
    try {
      const res = await mgmtFetch('/v0/management/memory/sessions?limit=200')
      const sessions = res.sessions || []
      setMemorySessions(sessions)
      if (!memorySession && sessions.length > 0) {
        setMemorySession(sessions[0].key)
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }, [mgmtFetch, showToast, memorySession])

  // Load session details
  const loadMemorySessionDetails = useCallback(async () => {
    if (!memorySession) {
      setMemoryDetails(null)
      return
    }
    try {
      const res = await mgmtFetch(`/v0/management/memory/session?session=${encodeURIComponent(memorySession)}`)
      const session = res.session || null
      setMemoryDetails(session)
      if (session) {
        setMemorySummary(session.summary || '')
        setMemoryPinned(session.pinned || '')
        setMemoryTodo(session.todo || '')
        setMemorySemanticEnabled(!session.semantic_disabled)
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }, [memorySession, mgmtFetch, showToast])

  // Load events
  const loadMemoryEvents = useCallback(async () => {
    if (!memorySession) {
      setMemoryEvents('Select a session.')
      return
    }
    try {
      const res = await mgmtFetch(`/v0/management/memory/events?session=${encodeURIComponent(memorySession)}&limit=${encodeURIComponent(memoryEventsLimit)}`)
      const events = res.events || []
      if (!Array.isArray(events) || events.length === 0) {
        setMemoryEvents('No events.')
        return
      }
      const lines = events.map((e: any) => {
        const ts = e.ts || ''
        const kind = e.kind || ''
        const role = e.role || ''
        const text = (e.text || '').toString()
        return `[${ts}][${kind}][${role}] ${text}`
      })
      setMemoryEvents(lines.join('\n\n'))
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }, [memorySession, memoryEventsLimit, mgmtFetch, showToast])

  // Load anchors
  const loadMemoryAnchors = useCallback(async () => {
    if (!memorySession) {
      setMemoryAnchors('Select a session.')
      return
    }
    try {
      const res = await mgmtFetch(`/v0/management/memory/anchors?session=${encodeURIComponent(memorySession)}&limit=${encodeURIComponent(memoryAnchorsLimit)}`)
      const anchors = res.anchors || []
      if (!Array.isArray(anchors) || anchors.length === 0) {
        setMemoryAnchors('No anchors.')
        return
      }
      const lines = anchors.map((a: any) => {
        const ts = a.ts || ''
        const summary = (a.summary || '').toString()
        return `[${ts}]\n${summary}`
      })
      setMemoryAnchors(lines.join('\n\n---\n\n'))
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }, [memorySession, memoryAnchorsLimit, mgmtFetch, showToast])

  // Save TODO
  const saveMemoryTodo = async () => {
    if (!memorySession) return
    try {
      await mgmtFetch('/v0/management/memory/todo', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: memorySession, value: memoryTodo }),
      })
      await loadMemorySessionDetails()
      showToast('Saved TODO', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Save pinned context
  const saveMemoryPinned = async () => {
    if (!memorySession) return
    try {
      await mgmtFetch('/v0/management/memory/pinned', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: memorySession, value: memoryPinned }),
      })
      await loadMemorySessionDetails()
      showToast('Saved pinned context', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Save summary
  const saveMemorySummary = async () => {
    if (!memorySession) return
    try {
      await mgmtFetch('/v0/management/memory/summary', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: memorySession, value: memorySummary }),
      })
      await loadMemorySessionDetails()
      showToast('Saved anchor summary', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Toggle semantic memory
  const toggleMemorySemantic = async (enabled: boolean) => {
    if (!memorySession) return
    try {
      await mgmtFetch('/v0/management/memory/semantic', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: memorySession, enabled }),
      })
      setMemorySemanticEnabled(enabled)
      await loadMemorySessionDetails()
      showToast(`Semantic ${enabled ? 'enabled' : 'disabled'}`, 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Delete session
  const deleteMemorySession = async () => {
    if (!memorySession) return
    try {
      await mgmtFetch(`/v0/management/memory/session?session=${encodeURIComponent(memorySession)}`, {
        method: 'DELETE',
      })
      setMemorySession('')
      setMemoryDetails(null)
      setMemoryEvents('')
      setMemoryAnchors('')
      await loadMemorySessions()
      showToast('Deleted session', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Export session
  const exportMemorySession = async () => {
    if (!memorySession || !mgmtKey) return
    try {
      const res = await fetch(`/v0/management/memory/export?session=${encodeURIComponent(memorySession)}`, {
        headers: { 'X-Management-Key': mgmtKey },
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg)
      }
      const blob = await res.blob()
      downloadBlob(blob, `proxypilot-session-${memorySession}.zip`)
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Export all sessions
  const exportAllMemory = async () => {
    if (!mgmtKey) return
    try {
      const res = await fetch('/v0/management/memory/export-all', {
        headers: { 'X-Management-Key': mgmtKey },
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg)
      }
      const blob = await res.blob()
      downloadBlob(blob, 'proxypilot-memory-all.zip')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Delete all sessions
  const deleteAllMemory = async () => {
    if (!mgmtKey) return
    if (!window.confirm('Delete all memory data? This cannot be undone.')) return
    try {
      await mgmtFetch('/v0/management/memory/delete-all?confirm=true', { method: 'POST' })
      setMemorySession('')
      setMemoryDetails(null)
      setMemoryEvents('')
      setMemoryAnchors('')
      await loadMemorySessions()
      showToast('Deleted all memory', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Import session
  const importMemorySession = async (file: File | null) => {
    if (!file || !memorySession || !mgmtKey) return
    try {
      const form = new FormData()
      form.append('file', file)
      const res = await fetch(`/v0/management/memory/import?session=${encodeURIComponent(memorySession)}&replace=${memoryImportReplace ? 'true' : 'false'}`, {
        method: 'POST',
        headers: { 'X-Management-Key': mgmtKey },
        body: form,
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg)
      }
      await loadMemorySessionDetails()
      showToast('Imported session', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Prune memory
  const pruneMemory = async () => {
    try {
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
      })
      await loadMemorySessions()
      showToast('Prune completed', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Load sessions on mount
  useEffect(() => {
    if (mgmtKey) {
      loadMemorySessions()
    }
  }, [mgmtKey, loadMemorySessions])

  if (!mgmtKey) {
    return null
  }

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
      <CardHeader className="pb-4">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-violet-500/10 flex items-center justify-center">
            <Database className="h-5 w-5 text-violet-500" />
          </div>
          <div>
            <CardTitle className="text-lg">Memory</CardTitle>
            <CardDescription>Anchors, pinned context, TODOs, and session history</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Action buttons */}
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" onClick={loadMemorySessions} className="gap-1">
            <RefreshCw className="h-3 w-3" />
            Refresh sessions
          </Button>
          <Button size="sm" variant="outline" onClick={loadMemorySessionDetails} className="gap-1">
            <FileText className="h-3 w-3" />
            Load session
          </Button>
          <Button size="sm" variant="outline" onClick={loadMemoryEvents} className="gap-1">
            <FileText className="h-3 w-3" />
            Events
          </Button>
          <Button size="sm" variant="outline" onClick={loadMemoryAnchors} className="gap-1">
            <Anchor className="h-3 w-3" />
            Anchors
          </Button>
          <Button size="sm" variant="outline" onClick={exportMemorySession} className="gap-1">
            <Download className="h-3 w-3" />
            Export
          </Button>
          <Button size="sm" variant="outline" onClick={exportAllMemory} className="gap-1">
            <Download className="h-3 w-3" />
            Export all
          </Button>
          <Button size="sm" variant="destructive" onClick={deleteMemorySession} className="gap-1">
            <Trash2 className="h-3 w-3" />
            Delete
          </Button>
          <Button size="sm" variant="destructive" onClick={deleteAllMemory} className="gap-1">
            <Trash2 className="h-3 w-3" />
            Delete all
          </Button>
        </div>

        {/* Session selector and limits */}
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
          <div className="flex items-center gap-1">
            <Label htmlFor="events-limit" className="text-xs text-muted-foreground">Events:</Label>
            <input
              id="events-limit"
              type="number"
              min={10}
              max={500}
              className="w-20 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
              value={memoryEventsLimit}
              onChange={(e) => setMemoryEventsLimit(parseInt(e.target.value || '120', 10))}
            />
          </div>
          <div className="flex items-center gap-1">
            <Label htmlFor="anchors-limit" className="text-xs text-muted-foreground">Anchors:</Label>
            <input
              id="anchors-limit"
              type="number"
              min={5}
              max={200}
              className="w-20 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
              value={memoryAnchorsLimit}
              onChange={(e) => setMemoryAnchorsLimit(parseInt(e.target.value || '20', 10))}
            />
          </div>
          <span className="text-xs text-muted-foreground">
            {memoryDetails?.updated_at ? `Updated ${memoryDetails.updated_at}` : 'No session loaded.'}
          </span>
          <div className="flex items-center gap-2">
            <Brain className="h-4 w-4 text-muted-foreground" />
            <Label htmlFor="semantic-toggle" className="text-xs text-muted-foreground cursor-pointer">
              Semantic
            </Label>
            <Switch
              id="semantic-toggle"
              checked={memorySemanticEnabled}
              disabled={!memorySession}
              onCheckedChange={toggleMemorySemantic}
            />
          </div>
        </div>

        <Separator className="bg-border/50" />

        {/* Session details: Summary, Pinned, TODO */}
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Anchor className="h-3 w-3" />
              Anchor summary
            </div>
            <textarea
              className="h-40 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono resize-y"
              value={memorySummary}
              onChange={(e) => setMemorySummary(e.target.value)}
              placeholder="Session anchor summary..."
            />
            <Button size="sm" variant="outline" onClick={saveMemorySummary} className="gap-1">
              <Save className="h-3 w-3" />
              Save summary
            </Button>
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Pin className="h-3 w-3" />
              Pinned context
            </div>
            <textarea
              className="h-40 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono resize-y"
              value={memoryPinned}
              onChange={(e) => setMemoryPinned(e.target.value)}
              placeholder="Pinned context that persists across turns..."
            />
            <Button size="sm" variant="outline" onClick={saveMemoryPinned} className="gap-1">
              <Save className="h-3 w-3" />
              Save pinned
            </Button>
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <ListTodo className="h-3 w-3" />
              TODO
            </div>
            <textarea
              className="h-40 w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono resize-y"
              value={memoryTodo}
              onChange={(e) => setMemoryTodo(e.target.value)}
              placeholder="Session TODO list..."
            />
            <Button size="sm" variant="outline" onClick={saveMemoryTodo} className="gap-1">
              <Save className="h-3 w-3" />
              Save TODO
            </Button>
          </div>
        </div>

        <Separator className="bg-border/50" />

        {/* Events and Anchors display */}
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <FileText className="h-3 w-3" />
              Events
            </div>
            <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs whitespace-pre-wrap">
              {memoryEvents || 'No events loaded.'}
            </pre>
          </div>
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Anchor className="h-3 w-3" />
              Anchors
            </div>
            <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs whitespace-pre-wrap">
              {memoryAnchors || 'No anchors loaded.'}
            </pre>
          </div>
        </div>

        <Separator className="bg-border/50" />

        {/* Import and Prune */}
        <div className="grid gap-4 lg:grid-cols-2">
          {/* Import section */}
          <div className="space-y-2 p-3 rounded-lg bg-muted/30 border border-border/50">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Upload className="h-3 w-3" />
              Import session (zip)
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <input
                type="file"
                accept=".zip"
                className="text-xs file:mr-2 file:py-1 file:px-2 file:rounded file:border-0 file:text-xs file:bg-primary file:text-primary-foreground hover:file:bg-primary/90"
                onChange={(e) => importMemorySession(e.target.files?.[0] || null)}
              />
              <div className="flex items-center gap-2">
                <Switch
                  id="import-replace"
                  checked={memoryImportReplace}
                  onCheckedChange={setMemoryImportReplace}
                />
                <Label htmlFor="import-replace" className="text-xs text-muted-foreground cursor-pointer">
                  Replace existing
                </Label>
              </div>
            </div>
          </div>

          {/* Prune section */}
          <div className="space-y-2 p-3 rounded-lg bg-muted/30 border border-border/50">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Scissors className="h-3 w-3" />
              Prune limits
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <Label htmlFor="prune-age" className="text-xs text-muted-foreground">Max age (days)</Label>
                <input
                  id="prune-age"
                  type="number"
                  min={0}
                  className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                  value={memoryPrune.maxAgeDays}
                  onChange={(e) => setMemoryPrune({ ...memoryPrune, maxAgeDays: parseInt(e.target.value || '0', 10) })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="prune-sessions" className="text-xs text-muted-foreground">Max sessions</Label>
                <input
                  id="prune-sessions"
                  type="number"
                  min={0}
                  className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                  value={memoryPrune.maxSessions}
                  onChange={(e) => setMemoryPrune({ ...memoryPrune, maxSessions: parseInt(e.target.value || '0', 10) })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="prune-namespaces" className="text-xs text-muted-foreground">Max namespaces</Label>
                <input
                  id="prune-namespaces"
                  type="number"
                  min={0}
                  className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                  value={memoryPrune.maxNamespaces}
                  onChange={(e) => setMemoryPrune({ ...memoryPrune, maxNamespaces: parseInt(e.target.value || '0', 10) })}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="prune-bytes-session" className="text-xs text-muted-foreground">Max bytes/session</Label>
                <input
                  id="prune-bytes-session"
                  type="number"
                  min={0}
                  className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                  value={memoryPrune.maxBytesPerSession}
                  onChange={(e) => setMemoryPrune({ ...memoryPrune, maxBytesPerSession: parseInt(e.target.value || '0', 10) })}
                />
              </div>
              <div className="space-y-1 col-span-2">
                <Label htmlFor="prune-bytes-namespace" className="text-xs text-muted-foreground">Max bytes/namespace</Label>
                <input
                  id="prune-bytes-namespace"
                  type="number"
                  min={0}
                  className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                  value={memoryPrune.maxBytesPerNamespace}
                  onChange={(e) => setMemoryPrune({ ...memoryPrune, maxBytesPerNamespace: parseInt(e.target.value || '0', 10) })}
                />
              </div>
            </div>
            <Button size="sm" variant="outline" onClick={pruneMemory} className="gap-1 mt-2">
              <Scissors className="h-3 w-3" />
              Prune memory
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
