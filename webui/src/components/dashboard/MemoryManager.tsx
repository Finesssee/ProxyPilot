import { useState, useEffect, useCallback, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import {
  Download,
  Upload,
  Trash2,
  RefreshCw,
  Save,
  Anchor,
  Pin,
  Scissors,
  Brain,
  ChevronDown,
  ChevronRight,
  Circle,
  AlertTriangle,
  Wrench,
  Loader2,
} from 'lucide-react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { cn } from '@/lib/utils'

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

interface ParsedAnchor {
  ts: string;
  summary: string;
}

interface ParsedEvent {
  ts: string;
  kind: string;
  role: string;
  text: string;
}

const downloadBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
};

// Format timestamp to HH:MM:SS
const formatTime = (ts: string): string => {
  if (!ts) return '--:--:--';
  try {
    const date = new Date(ts);
    return date.toTimeString().slice(0, 8);
  } catch {
    return ts.slice(11, 19) || ts;
  }
};

// Calculate duration between two timestamps
const calculateDuration = (start: string, end: string): string => {
  try {
    const startDate = new Date(start);
    const endDate = new Date(end);
    const diffMs = endDate.getTime() - startDate.getTime();
    const hours = Math.floor(diffMs / 3600000);
    const minutes = Math.floor((diffMs % 3600000) / 60000);
    const seconds = Math.floor((diffMs % 60000) / 1000);
    return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
  } catch {
    return '--:--:--';
  }
};

// Collapsible section component with smooth animation
function CollapsibleSection({
  title,
  icon: Icon,
  count,
  children,
  defaultOpen = false,
}: {
  title: string;
  icon: React.ElementType;
  count?: number;
  children: React.ReactNode;
  defaultOpen?: boolean;
}) {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const contentRef = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState<number | undefined>(defaultOpen ? undefined : 0);

  useEffect(() => {
    if (isOpen) {
      const contentEl = contentRef.current;
      if (contentEl) {
        setHeight(contentEl.scrollHeight);
        // After animation, set to auto for dynamic content
        const timer = setTimeout(() => setHeight(undefined), 300);
        return () => clearTimeout(timer);
      }
    } else {
      setHeight(0);
    }
  }, [isOpen]);

  return (
    <div className="border border-orange-500/30 rounded bg-black/40">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-orange-500/10 transition-colors"
      >
        {isOpen ? (
          <ChevronDown className="h-4 w-4 text-orange-400" />
        ) : (
          <ChevronRight className="h-4 w-4 text-orange-400" />
        )}
        <Icon className="h-4 w-4 text-orange-400" />
        <span className="text-xs font-mono text-orange-300 uppercase tracking-wider">
          {title}
        </span>
        {count !== undefined && (
          <span className="ml-auto text-xs font-mono bg-orange-500/20 text-orange-300 px-2 py-0.5 rounded">
            {count}
          </span>
        )}
      </button>
      <div
        style={{ height: height !== undefined ? `${height}px` : 'auto' }}
        className="overflow-hidden transition-[height] duration-300 ease-in-out"
      >
        <div ref={contentRef} className="px-3 pb-3">
          {children}
        </div>
      </div>
    </div>
  );
}

export function MemoryManager() {
  const { mgmtKey, mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()

  // Session state
  const [memorySessions, setMemorySessions] = useState<MemorySession[]>([])
  const [memorySession, setMemorySession] = useState('')
  const [memoryDetails, setMemoryDetails] = useState<MemorySession | null>(null)

  // Session details
  const [memorySummary, setMemorySummary] = useState('')
  const [memoryPinned, setMemoryPinned] = useState('')
  const [memoryTodo, setMemoryTodo] = useState('')
  const [memorySemanticEnabled, setMemorySemanticEnabled] = useState(true)

  // Events and anchors - now parsed
  const [memoryEvents, setMemoryEvents] = useState<ParsedEvent[]>([])
  const [memoryAnchors, setMemoryAnchors] = useState<ParsedAnchor[]>([])
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

  // Session dropdown open
  const [sessionDropdownOpen, setSessionDropdownOpen] = useState(false)

  const isRunning = status?.running ?? false

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
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
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
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }, [memorySession, mgmtFetch, showToast])

  // Load events
  const loadMemoryEvents = useCallback(async () => {
    if (!memorySession) {
      setMemoryEvents([])
      return
    }
    try {
      const res = await mgmtFetch(`/v0/management/memory/events?session=${encodeURIComponent(memorySession)}&limit=${encodeURIComponent(memoryEventsLimit)}`)
      const events = res.events || []
      if (!Array.isArray(events) || events.length === 0) {
        setMemoryEvents([])
        return
      }
      const parsed: ParsedEvent[] = events.map((e: any) => ({
        ts: e.ts || '',
        kind: e.kind || '',
        role: e.role || '',
        text: (e.text || '').toString(),
      }))
      setMemoryEvents(parsed)
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }, [memorySession, memoryEventsLimit, mgmtFetch, showToast])

  // Load anchors
  const loadMemoryAnchors = useCallback(async () => {
    if (!memorySession) {
      setMemoryAnchors([])
      return
    }
    try {
      const res = await mgmtFetch(`/v0/management/memory/anchors?session=${encodeURIComponent(memorySession)}&limit=${encodeURIComponent(memoryAnchorsLimit)}`)
      const anchors = res.anchors || []
      if (!Array.isArray(anchors) || anchors.length === 0) {
        setMemoryAnchors([])
        return
      }
      const parsed: ParsedAnchor[] = anchors.map((a: any) => ({
        ts: a.ts || '',
        summary: (a.summary || '').toString(),
      }))
      setMemoryAnchors(parsed)
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
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
      setMemoryEvents([])
      setMemoryAnchors([])
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
      downloadBlob(blob, `flight-log-${memorySession}.zip`)
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
      downloadBlob(blob, 'flight-data-complete.zip')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Delete all sessions
  const deleteAllMemory = async () => {
    if (!mgmtKey) return
    if (!window.confirm('PURGE ALL FLIGHT DATA? This action cannot be reversed.')) return
    try {
      await mgmtFetch('/v0/management/memory/delete-all?confirm=true', { method: 'POST' })
      setMemorySession('')
      setMemoryDetails(null)
      setMemoryEvents([])
      setMemoryAnchors([])
      await loadMemorySessions()
      showToast('All flight data purged', 'success')
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
      showToast('Flight data imported', 'success')
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
      showToast('Maintenance complete', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // Load sessions on mount
  useEffect(() => {
    if (mgmtKey && isRunning) {
      loadMemorySessions()
    } else if (!isRunning) {
      setMemorySessions([])
      setMemoryDetails(null)
      setMemoryEvents([])
      setMemoryAnchors([])
    }
  }, [mgmtKey, isRunning, loadMemorySessions])

  // Auto-load session details when session changes
  useEffect(() => {
    if (memorySession && isRunning) {
      loadMemorySessionDetails()
      loadMemoryAnchors()
      loadMemoryEvents()
    }
  }, [memorySession, isRunning, loadMemorySessionDetails, loadMemoryAnchors, loadMemoryEvents])

  if (!mgmtKey) {
    return null
  }

  // Get session start time and calculate duration
  const sessionStartTime = memoryEvents.length > 0 ? memoryEvents[memoryEvents.length - 1]?.ts : '';
  const sessionEndTime = memoryEvents.length > 0 ? memoryEvents[0]?.ts : '';
  const sessionDuration = sessionStartTime && sessionEndTime ? calculateDuration(sessionStartTime, sessionEndTime) : '--:--:--';

  return (
    <div className="bg-gradient-to-b from-zinc-900 to-black border border-orange-500/40 rounded-lg shadow-2xl shadow-orange-500/10 overflow-hidden">
      {/* Header */}
      <div className="bg-gradient-to-r from-orange-950/80 via-orange-900/60 to-orange-950/80 border-b border-orange-500/40 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="relative">
              <Circle className={`h-4 w-4 ${isRunning ? 'text-red-500 fill-red-500 animate-pulse' : 'text-muted'}`} />
              {isRunning && <div className="absolute inset-0 h-4 w-4 bg-red-500 rounded-full blur-sm opacity-50 animate-pulse" />}
            </div>
            <span className="text-sm font-mono font-bold text-orange-300 uppercase tracking-widest">
              Black Box Recorder
            </span>
            {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-orange-500/60" />}
            <span className="text-xs font-mono text-orange-500/60">
              [{memorySessions.length} LOGS]
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={exportMemorySession}
              disabled={!memorySession || !isRunning}
              className="gap-1.5 text-xs font-mono bg-orange-500/10 hover:bg-orange-500/20 text-orange-300 border border-orange-500/30"
            >
              <Download className="h-3 w-3" />
              EXPORT
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={exportAllMemory}
              disabled={!isRunning}
              className="gap-1.5 text-xs font-mono bg-orange-500/10 hover:bg-orange-500/20 text-orange-300 border border-orange-500/30"
            >
              <Download className="h-3 w-3" />
              EXPORT ALL
            </Button>
            <label className={cn("cursor-pointer", !isRunning && "opacity-50 pointer-events-none")}>
              <input
                type="file"
                accept=".zip"
                className="hidden"
                disabled={!isRunning}
                onChange={(e) => importMemorySession(e.target.files?.[0] || null)}
              />
              <div className="flex items-center gap-1.5 text-xs font-mono bg-orange-500/10 hover:bg-orange-500/20 text-orange-300 border border-orange-500/30 px-3 py-1.5 rounded-md transition-colors">
                <Upload className="h-3 w-3" />
                IMPORT
              </div>
            </label>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="p-4 space-y-4">
        {!isRunning && (
          <div className="py-8 text-center text-muted-foreground border border-dashed border-orange-500/20 rounded bg-black/20">
            <p className="text-sm uppercase tracking-widest font-mono">⚠️ Engine Offline</p>
            <p className="text-xs mt-1">Start the proxy engine to access flight data</p>
          </div>
        )}

        {isRunning && (
          <>
            {/* Session Selector */}
            <div className="space-y-3">
              <div className="flex items-center gap-3">
                <span className="text-xs font-mono text-orange-400 uppercase tracking-wider">Session:</span>
                <div className="relative flex-1">
                  <button
                    onClick={() => setSessionDropdownOpen(!sessionDropdownOpen)}
                    className="w-full flex items-center justify-between px-3 py-2 bg-black/60 border border-orange-500/30 rounded text-left hover:border-orange-500/50 transition-colors"
                  >
                    <span className="font-mono text-sm text-orange-200">
                      {memorySession || '(no session selected)'}
                    </span>
                    <ChevronDown className={`h-4 w-4 text-orange-400 transition-transform ${sessionDropdownOpen ? 'rotate-180' : ''}`} />
                  </button>
                  {sessionDropdownOpen && (
                    <div className="absolute z-10 w-full mt-1 max-h-60 overflow-auto bg-zinc-900 border border-orange-500/30 rounded shadow-xl">
                      {memorySessions.length === 0 ? (
                        <div className="px-3 py-2 text-xs font-mono text-orange-500/60">(no sessions)</div>
                      ) : (
                        memorySessions.map((s) => (
                          <button
                            key={s.key}
                            onClick={() => {
                              setMemorySession(s.key)
                              setSessionDropdownOpen(false)
                            }}
                            className={`w-full px-3 py-2 text-left text-sm font-mono hover:bg-orange-500/10 transition-colors ${s.key === memorySession ? 'bg-orange-500/20 text-orange-200' : 'text-orange-300/80'
                              }`}
                          >
                            {s.key}
                          </button>
                        ))
                      )}
                    </div>
                  )}
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={loadMemorySessions}
                  className="text-orange-400 hover:text-orange-300 hover:bg-orange-500/10"
                >
                  <RefreshCw className="h-4 w-4" />
                </Button>
              </div>

              {/* Session Metadata Card */}
              {memorySession && (
                <div className="bg-black/40 border border-orange-500/20 rounded p-3">
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs font-mono">
                    <div>
                      <span className="text-orange-500/60">Started:</span>
                      <div className="text-orange-200">{formatTime(sessionStartTime)} UTC</div>
                    </div>
                    <div>
                      <span className="text-orange-500/60">Duration:</span>
                      <div className="text-orange-200">{sessionDuration}</div>
                    </div>
                    <div>
                      <span className="text-orange-500/60">Anchors:</span>
                      <div className="text-orange-200">{memoryAnchors.length}</div>
                    </div>
                    <div>
                      <span className="text-orange-500/60">Events:</span>
                      <div className="text-orange-200">{memoryEvents.length}</div>
                    </div>
                  </div>
                  <div className="flex items-center gap-4 mt-3 pt-3 border-t border-orange-500/20">
                    <div className="flex items-center gap-2">
                      <Brain className="h-4 w-4 text-orange-400" />
                      <Label htmlFor="semantic-toggle" className="text-xs font-mono text-orange-300 cursor-pointer">
                        SEMANTIC
                      </Label>
                      <Switch
                        id="semantic-toggle"
                        checked={memorySemanticEnabled}
                        disabled={!memorySession}
                        onCheckedChange={toggleMemorySemantic}
                      />
                    </div>
                    <span className="text-xs font-mono text-orange-500/60">
                      {memoryDetails?.updated_at ? `Last update: ${memoryDetails.updated_at}` : ''}
                    </span>
                  </div>
                </div>
              )}
            </div>

            {/* Anchors Section */}
            <CollapsibleSection
              title="Anchors"
              icon={Anchor}
              count={memoryAnchors.length}
              defaultOpen={true}
            >
              <div className="space-y-2">
                <div className="flex items-center gap-2 mb-2">
                  <Label className="text-xs font-mono text-orange-500/60">Limit:</Label>
                  <input
                    type="number"
                    min={5}
                    max={200}
                    className="w-16 rounded bg-black/60 border border-orange-500/30 px-2 py-1 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                    value={memoryAnchorsLimit}
                    onChange={(e) => setMemoryAnchorsLimit(parseInt(e.target.value || '20', 10))}
                  />
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={loadMemoryAnchors}
                    className="text-xs font-mono text-orange-400 hover:bg-orange-500/10"
                  >
                    <RefreshCw className="h-3 w-3 mr-1" />
                    Reload
                  </Button>
                </div>
                <div className="bg-black/60 border border-orange-500/20 rounded max-h-48 overflow-auto">
                  {memoryAnchors.length === 0 ? (
                    <div className="px-3 py-4 text-xs font-mono text-orange-500/50 text-center">No anchors recorded.</div>
                  ) : (
                    <div className="divide-y divide-orange-500/10">
                      {memoryAnchors.map((anchor, i) => (
                        <div key={i} className="px-3 py-2 hover:bg-orange-500/5">
                          <div className="flex items-start gap-2">
                            <span className="text-xs font-mono text-orange-400 shrink-0">
                              {formatTime(anchor.ts)}
                            </span>
                            <span className="text-xs font-mono text-orange-500/30">|</span>
                            <span className="text-xs font-mono text-orange-200/80 break-words">
                              {anchor.summary.length > 100 ? anchor.summary.slice(0, 100) + '...' : anchor.summary}
                            </span>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
                <div className="space-y-2 mt-3">
                  <Label className="text-xs font-mono text-orange-500/60">Anchor Summary:</Label>
                  <textarea
                    className="w-full h-24 rounded bg-black/60 border border-orange-500/30 px-3 py-2 text-xs font-mono text-orange-200 resize-y focus:border-orange-500/50 focus:outline-none"
                    value={memorySummary}
                    onChange={(e) => setMemorySummary(e.target.value)}
                    placeholder="Session anchor summary..."
                  />
                  <Button
                    size="sm"
                    onClick={saveMemorySummary}
                    className="gap-1 text-xs font-mono bg-orange-500/20 hover:bg-orange-500/30 text-orange-300 border border-orange-500/30"
                  >
                    <Save className="h-3 w-3" />
                    Save Summary
                  </Button>
                </div>
              </div>
            </CollapsibleSection>

            {/* Pinned Context Section */}
            <CollapsibleSection
              title="Pinned Context"
              icon={Pin}
              defaultOpen={false}
            >
              <div className="space-y-2">
                <textarea
                  className="w-full h-32 rounded bg-black/60 border border-orange-500/30 px-3 py-2 text-xs font-mono text-orange-200 resize-y focus:border-orange-500/50 focus:outline-none"
                  value={memoryPinned}
                  onChange={(e) => setMemoryPinned(e.target.value)}
                  placeholder="Pinned context that persists across turns..."
                />
                <Button
                  size="sm"
                  onClick={saveMemoryPinned}
                  className="gap-1 text-xs font-mono bg-orange-500/20 hover:bg-orange-500/30 text-orange-300 border border-orange-500/30"
                >
                  <Save className="h-3 w-3" />
                  Save Pinned
                </Button>
              </div>
            </CollapsibleSection>

            {/* TODO Section */}
            <CollapsibleSection
              title="TODO Log"
              icon={Anchor}
              defaultOpen={false}
            >
              <div className="space-y-2">
                <textarea
                  className="w-full h-32 rounded bg-black/60 border border-orange-500/30 px-3 py-2 text-xs font-mono text-orange-200 resize-y focus:border-orange-500/50 focus:outline-none"
                  value={memoryTodo}
                  onChange={(e) => setMemoryTodo(e.target.value)}
                  placeholder="Session TODO list..."
                />
                <Button
                  size="sm"
                  onClick={saveMemoryTodo}
                  className="gap-1 text-xs font-mono bg-orange-500/20 hover:bg-orange-500/30 text-orange-300 border border-orange-500/30"
                >
                  <Save className="h-3 w-3" />
                  Save TODO
                </Button>
              </div>
            </CollapsibleSection>

            {/* Events Section */}
            <CollapsibleSection
              title="Event Log"
              icon={Circle}
              count={memoryEvents.length}
              defaultOpen={false}
            >
              <div className="space-y-2">
                <div className="flex items-center gap-2 mb-2">
                  <Label className="text-xs font-mono text-orange-500/60">Limit:</Label>
                  <input
                    type="number"
                    min={10}
                    max={500}
                    className="w-16 rounded bg-black/60 border border-orange-500/30 px-2 py-1 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                    value={memoryEventsLimit}
                    onChange={(e) => setMemoryEventsLimit(parseInt(e.target.value || '120', 10))}
                  />
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={loadMemoryEvents}
                    className="text-xs font-mono text-orange-400 hover:bg-orange-500/10"
                  >
                    <RefreshCw className="h-3 w-3 mr-1" />
                    Reload
                  </Button>
                </div>
                <div className="bg-black/60 border border-orange-500/20 rounded max-h-64 overflow-auto">
                  {memoryEvents.length === 0 ? (
                    <div className="px-3 py-4 text-xs font-mono text-orange-500/50 text-center">No events recorded.</div>
                  ) : (
                    <div className="divide-y divide-orange-500/10">
                      {memoryEvents.map((event, i) => (
                        <div key={i} className="px-3 py-2 hover:bg-orange-500/5">
                          <div className="flex items-start gap-2">
                            <span className="text-xs font-mono text-orange-400 shrink-0">
                              {formatTime(event.ts)}
                            </span>
                            <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-orange-500/20 text-orange-300 shrink-0">
                              {event.kind}
                            </span>
                            <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-zinc-700/50 text-zinc-300 shrink-0">
                              {event.role}
                            </span>
                            <span className="text-xs font-mono text-orange-200/70 break-words">
                              {event.text.length > 80 ? event.text.slice(0, 80) + '...' : event.text}
                            </span>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </CollapsibleSection>

            {/* Maintenance Panel */}
            <div className="relative pt-6">
              <div className="absolute top-0 left-0 right-0 flex items-center">
                <div className="flex-1 border-t border-dashed border-orange-500/30" />
                <span className="px-3 text-xs font-mono text-orange-500/60 uppercase tracking-widest flex items-center gap-2">
                  <Wrench className="h-3 w-3" />
                  Maintenance
                </span>
                <div className="flex-1 border-t border-dashed border-orange-500/30" />
              </div>

              <div className="bg-black/40 border border-orange-500/20 rounded p-4 space-y-4">
                {/* Import Options */}
                <div className="flex items-center gap-4 pb-3 border-b border-orange-500/20">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="import-replace"
                      checked={memoryImportReplace}
                      onCheckedChange={setMemoryImportReplace}
                    />
                    <Label htmlFor="import-replace" className="text-xs font-mono text-orange-300 cursor-pointer">
                      Replace on Import
                    </Label>
                  </div>
                </div>

                {/* Prune Settings */}
                <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                  <div className="space-y-1">
                    <Label className="text-xs font-mono text-orange-500/60">Max Sessions</Label>
                    <input
                      type="number"
                      min={0}
                      className="w-full rounded bg-black/60 border border-orange-500/30 px-2 py-1.5 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                      value={memoryPrune.maxSessions}
                      onChange={(e) => setMemoryPrune({ ...memoryPrune, maxSessions: parseInt(e.target.value || '0', 10) })}
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs font-mono text-orange-500/60">Max Age (days)</Label>
                    <input
                      type="number"
                      min={0}
                      className="w-full rounded bg-black/60 border border-orange-500/30 px-2 py-1.5 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                      value={memoryPrune.maxAgeDays}
                      onChange={(e) => setMemoryPrune({ ...memoryPrune, maxAgeDays: parseInt(e.target.value || '0', 10) })}
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs font-mono text-orange-500/60">Max Namespaces</Label>
                    <input
                      type="number"
                      min={0}
                      className="w-full rounded bg-black/60 border border-orange-500/30 px-2 py-1.5 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                      value={memoryPrune.maxNamespaces}
                      onChange={(e) => setMemoryPrune({ ...memoryPrune, maxNamespaces: parseInt(e.target.value || '0', 10) })}
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs font-mono text-orange-500/60">Bytes/Session</Label>
                    <input
                      type="number"
                      min={0}
                      className="w-full rounded bg-black/60 border border-orange-500/30 px-2 py-1.5 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                      value={memoryPrune.maxBytesPerSession}
                      onChange={(e) => setMemoryPrune({ ...memoryPrune, maxBytesPerSession: parseInt(e.target.value || '0', 10) })}
                    />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs font-mono text-orange-500/60">Bytes/Namespace</Label>
                    <input
                      type="number"
                      min={0}
                      className="w-full rounded bg-black/60 border border-orange-500/30 px-2 py-1.5 text-xs font-mono text-orange-200 focus:border-orange-500/50 focus:outline-none"
                      value={memoryPrune.maxBytesPerNamespace}
                      onChange={(e) => setMemoryPrune({ ...memoryPrune, maxBytesPerNamespace: parseInt(e.target.value || '0', 10) })}
                    />
                  </div>
                </div>

                {/* Action Buttons */}
                <div className="flex items-center gap-3 pt-2">
                  <Button
                    size="sm"
                    onClick={pruneMemory}
                    className="gap-1.5 text-xs font-mono bg-amber-600/80 hover:bg-amber-600 text-black border border-amber-500"
                  >
                    <Scissors className="h-3 w-3" />
                    PRUNE NOW
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={deleteMemorySession}
                    disabled={!memorySession}
                    className="gap-1.5 text-xs font-mono bg-red-500/20 hover:bg-red-500/30 text-red-400 border border-red-500/30"
                  >
                    <Trash2 className="h-3 w-3" />
                    Delete Session
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={deleteAllMemory}
                    className="gap-1.5 text-xs font-mono bg-red-500/20 hover:bg-red-500/30 text-red-400 border border-red-500/30"
                  >
                    <AlertTriangle className="h-3 w-3" />
                    Purge All Data
                  </Button>
                </div>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
