import { useState, useRef, useEffect, useCallback } from 'react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { Card, CardHeader, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Copy, Trash2, FileText, Circle, Download, Loader2 } from 'lucide-react'

type LogLevel = 'ERROR' | 'WARN' | 'INFO' | 'DEBUG' | 'ALL'

interface ParsedLogEntry {
  id: string
  timestamp: string
  level: LogLevel
  message: string
  raw: string
}

// Parse log line to extract timestamp, level, and message
function parseLogLine(line: string, index: number): ParsedLogEntry {
  // Common log patterns: [LEVEL] message, LEVEL: message, timestamp LEVEL message
  const patterns = [
    // Pattern: [2024-01-15 18:45:23.456] [INFO] message
    /^\[?(\d{4}-\d{2}-\d{2}\s+)?(\d{2}:\d{2}:\d{2}(?:\.\d{3})?)\]?\s*\[?(ERROR|WARN(?:ING)?|INFO|DEBUG)\]?:?\s*(.*)$/i,
    // Pattern: ERROR: message or [ERROR] message
    /^\[?(ERROR|WARN(?:ING)?|INFO|DEBUG)\]?:?\s*(.*)$/i,
    // Pattern with timestamp prefix: 18:45:23 INFO message
    /^(\d{2}:\d{2}:\d{2}(?:\.\d{3})?)\s+(ERROR|WARN(?:ING)?|INFO|DEBUG)\s+(.*)$/i,
  ]

  const now = new Date()
  const defaultTimestamp = now.toTimeString().slice(0, 12).replace(',', '.')

  for (const pattern of patterns) {
    const match = line.match(pattern)
    if (match) {
      if (pattern === patterns[0]) {
        // Full pattern with optional date and time
        const timestamp = match[2] || defaultTimestamp
        const level = normalizeLevel(match[3])
        const message = match[4] || ''
        return { id: `log-${index}`, timestamp: formatTimestamp(timestamp), level, message, raw: line }
      } else if (pattern === patterns[1]) {
        // Level only pattern
        const level = normalizeLevel(match[1])
        const message = match[2] || ''
        return { id: `log-${index}`, timestamp: defaultTimestamp, level, message, raw: line }
      } else if (pattern === patterns[2]) {
        // Timestamp + level pattern
        const timestamp = match[1]
        const level = normalizeLevel(match[2])
        const message = match[3] || ''
        return { id: `log-${index}`, timestamp: formatTimestamp(timestamp), level, message, raw: line }
      }
    }
  }

  // Default: treat as INFO message
  return { id: `log-${index}`, timestamp: defaultTimestamp, level: 'INFO', message: line, raw: line }
}

function normalizeLevel(level: string): LogLevel {
  const upper = level.toUpperCase()
  if (upper === 'WARNING') return 'WARN'
  if (['ERROR', 'WARN', 'INFO', 'DEBUG'].includes(upper)) return upper as LogLevel
  return 'INFO'
}

function formatTimestamp(ts: string): string {
  // Ensure HH:MM:SS.mmm format
  if (ts.includes('.')) {
    const [time, ms] = ts.split('.')
    return `${time}.${ms.padEnd(3, '0').slice(0, 3)}`
  }
  return `${ts}.000`
}

function getLevelBorderColor(level: LogLevel): string {
  switch (level) {
    case 'ERROR': return 'border-l-[var(--status-offline)]'
    case 'WARN': return 'border-l-[var(--status-warning)]'
    case 'INFO': return 'border-l-[var(--accent-glow)]'
    case 'DEBUG': return 'border-l-[var(--text-muted)]'
    default: return 'border-l-[var(--text-muted)]'
  }
}

function getLevelBadgeStyle(level: LogLevel): string {
  switch (level) {
    case 'ERROR': return 'bg-[var(--status-offline)]/20 text-[var(--status-offline)]'
    case 'WARN': return 'bg-[var(--status-warning)]/20 text-[var(--status-warning)]'
    case 'INFO': return 'bg-[var(--accent-glow)]/20 text-[var(--accent-glow)]'
    case 'DEBUG': return 'bg-[var(--text-muted)]/20 text-[var(--text-muted)]'
    default: return 'bg-[var(--text-muted)]/20 text-[var(--text-muted)]'
  }
}

export function LogsViewer() {
  const { mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()
  const [diagnostics, setDiagnostics] = useState('')
  const [logText, setLogText] = useState('')
  const [activeLogType, setActiveLogType] = useState<'stdout' | 'stderr'>('stdout')
  const [activeFilter, setActiveFilter] = useState<LogLevel>('ALL')
  const [isLive, setIsLive] = useState(false)
  const [autoScroll, setAutoScroll] = useState(true)
  const logsContainerRef = useRef<HTMLDivElement>(null)
  const liveIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const isRunning = status?.running ?? false

  const loadDiagnostics = async () => {
    try {
      const res = await mgmtFetch('/v0/management/proxypilot/diagnostics?lines=120')
      setDiagnostics(res.text || '')
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }

  const copyDiagnostics = async () => {
    if (!diagnostics) {
      showToast('No diagnostics to copy', 'error')
      return
    }
    try {
      await navigator.clipboard.writeText(diagnostics)
      showToast('Copied diagnostics to clipboard', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const tailLogs = useCallback(async (kind: 'stdout' | 'stderr') => {
    try {
      setActiveLogType(kind)
      const res = await mgmtFetch(`/v0/management/proxypilot/logs/tail?file=${kind}&lines=200`)
      setLogText((res.lines || []).join('\n'))
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }, [mgmtFetch, showToast])

  const copyLogs = async () => {
    if (!logText) {
      showToast('No logs to copy', 'error')
      return
    }
    try {
      await navigator.clipboard.writeText(logText)
      showToast('Copied logs to clipboard', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const clearLogs = () => {
    setLogText('')
    setIsLive(false)
    if (liveIntervalRef.current) {
      clearInterval(liveIntervalRef.current)
      liveIntervalRef.current = null
    }
  }

  const downloadLogs = async () => {
    try {
      const res = await mgmtFetch(`/v0/management/proxypilot/logs/tail?file=${activeLogType}&lines=2000`)
      const text = (res.lines || []).join('\n')
      if (!text) {
        showToast('No logs available to download', 'error')
        return
      }
      const blob = new Blob([text], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `proxypilot-${activeLogType}-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.log`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      showToast('Logs download started', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const toggleLive = () => {
    if (isLive) {
      setIsLive(false)
      if (liveIntervalRef.current) {
        clearInterval(liveIntervalRef.current)
        liveIntervalRef.current = null
      }
    } else {
      setIsLive(true)
      tailLogs(activeLogType)
      liveIntervalRef.current = setInterval(() => {
        tailLogs(activeLogType)
      }, 2000)
    }
  }

  // Auto-scroll when new logs arrive
  useEffect(() => {
    if (autoScroll && logsContainerRef.current) {
      logsContainerRef.current.scrollTop = logsContainerRef.current.scrollHeight
    }
  }, [logText, autoScroll])

  // Cleanup interval on unmount
  useEffect(() => {
    return () => {
      if (liveIntervalRef.current) {
        clearInterval(liveIntervalRef.current)
      }
    }
  }, [])

  // Parse log entries
  const logEntries: ParsedLogEntry[] = logText
    ? logText.split('\n').filter(Boolean).map((line, i) => parseLogLine(line, i))
    : []

  // Filter log entries
  const filteredEntries = activeFilter === 'ALL'
    ? logEntries
    : logEntries.filter(entry => entry.level === activeFilter)

  const filterButtons: LogLevel[] = ['ALL', 'ERROR', 'WARN', 'INFO', 'DEBUG']

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* Diagnostics Card */}
      <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-xl bg-blue-500/10 flex items-center justify-center">
                <FileText className="h-5 w-5 text-blue-500" />
              </div>
              <div>
                <h3 className="text-lg font-semibold">Diagnostics</h3>
                <p className="text-sm text-muted-foreground">Engine snapshot</p>
              </div>
            </div>
            {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {!isRunning && (
            <div className="py-4 text-center text-muted-foreground">
              <p className="text-xs uppercase tracking-widest font-mono">⚠️ Engine Offline</p>
            </div>
          )}
          {isRunning && (
            <>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={loadDiagnostics}>
                  <Download className="h-4 w-4 mr-1" />
                  Load
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={copyDiagnostics}
                  disabled={!diagnostics}
                  className="gap-2"
                >
                  <Copy className="h-4 w-4" />
                  Copy
                </Button>
              </div>
              <pre className="max-h-48 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs font-mono">
                {diagnostics || 'No diagnostics loaded.'}
              </pre>
            </>
          )}
        </CardContent>
      </Card>

      {/* Flight Recorder Logs Card */}
      <Card className="backdrop-blur-sm bg-[var(--bg-void)] border-[var(--border-default)] shadow-xl overflow-hidden">
        {/* Flight Recorder Header */}
        <CardHeader className="pb-3 border-b border-[var(--border-subtle)] bg-[var(--bg-panel)]">
          <div className="flex items-center justify-between flex-wrap gap-3">
            {/* Title with record indicator */}
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2">
                <Circle
                  className={`h-3 w-3 fill-current ${isLive && isRunning ? 'text-[var(--status-offline)] animate-pulse' : 'text-[var(--text-muted)]'}`}
                />
                <span className="font-mono text-sm font-semibold tracking-wider text-[var(--text-primary)]">
                  FLIGHT RECORDER
                </span>
              </div>
              {isMgmtLoading && <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />}

              {/* LIVE indicator */}
              <button
                onClick={toggleLive}
                disabled={!isRunning}
                className={`flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-mono transition-all ${isLive && isRunning
                  ? 'bg-[var(--status-offline)]/20 text-[var(--status-offline)] animate-pulse'
                  : 'bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
                  } ${!isRunning ? 'opacity-50 cursor-not-allowed' : ''}`}
              >
                <span className={`h-1.5 w-1.5 rounded-full ${isLive && isRunning ? 'bg-[var(--status-offline)]' : 'bg-[var(--text-muted)]'}`} />
                LIVE
              </button>
            </div>

            {/* Filter chips */}
            <div className="flex items-center gap-1">
              {filterButtons.map(level => (
                <button
                  key={level}
                  onClick={() => setActiveFilter(level)}
                  className={`px-2 py-0.5 rounded text-xs font-mono uppercase transition-all ${activeFilter === level
                    ? level === 'ALL'
                      ? 'bg-[var(--accent-primary)] text-[var(--primary-foreground)]'
                      : getLevelBadgeStyle(level).replace('/20', '/40')
                    : 'bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
                    }`}
                >
                  {level === 'ALL' ? 'ALL' : level.slice(0, 3)}
                </button>
              ))}
            </div>
          </div>

          {/* Secondary controls */}
          <div className="flex items-center gap-2 mt-3">
            <Button
              size="sm"
              variant={activeLogType === 'stdout' ? 'default' : 'outline'}
              onClick={() => tailLogs('stdout')}
              className="text-xs h-7 font-mono"
            >
              STDOUT
            </Button>
            <Button
              size="sm"
              variant={activeLogType === 'stderr' ? 'default' : 'outline'}
              onClick={() => tailLogs('stderr')}
              className="text-xs h-7 font-mono"
            >
              STDERR
            </Button>
            <div className="flex-1" />
            <Button
              size="sm"
              variant="ghost"
              onClick={copyLogs}
              disabled={!logText}
              className="text-xs h-7 gap-1 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
            >
              <Copy className="h-3 w-3" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={downloadLogs}
              disabled={!logText}
              className="text-xs h-7 gap-1 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
            >
              <Download className="h-3 w-3" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={clearLogs}
              disabled={!logText}
              className="text-xs h-7 gap-1 text-[var(--text-muted)] hover:text-[var(--status-offline)]"
            >
              <Trash2 className="h-3 w-3" />
            </Button>
          </div>
        </CardHeader>

        {/* Log entries with scanline effect */}
        <CardContent className="p-0 relative">
          {/* Scanline overlay */}
          <div
            className="absolute inset-0 pointer-events-none z-10 overflow-hidden"
            aria-hidden="true"
          >
            <div
              className="absolute left-0 right-0 h-8 bg-gradient-to-b from-[var(--accent-glow)]/[0.07] to-transparent animate-scanline"
              style={{ top: '-10%' }}
            />
          </div>

          {/* Log content */}
          <div
            ref={logsContainerRef}
            className="max-h-64 overflow-auto bg-[var(--bg-void)] font-mono text-xs"
            onScroll={(e) => {
              const target = e.target as HTMLDivElement
              const isAtBottom = target.scrollHeight - target.scrollTop <= target.clientHeight + 50
              setAutoScroll(isAtBottom)
            }}
          >
            {!isRunning ? (
              <div className="p-8 text-center text-[var(--text-muted)]">
                <p className="text-xs uppercase tracking-widest font-mono">⚠️ Engine Offline</p>
                <p className="text-[10px] mt-1">Start the proxy engine to view logs</p>
              </div>
            ) : filteredEntries.length === 0 ? (
              <div className="p-4 text-center text-[var(--text-muted)]">
                {logText ? 'No matching log entries.' : 'No logs loaded. Click STDOUT or STDERR to load logs.'}
              </div>
            ) : (
              <div className="divide-y divide-[var(--border-subtle)]/30">
                {filteredEntries.map((entry) => (
                  <div
                    key={entry.id}
                    className={`flex items-start gap-2 px-3 py-1.5 border-l-2 hover:bg-[var(--bg-panel)]/50 transition-colors ${getLevelBorderColor(entry.level)}`}
                  >
                    {/* Timestamp */}
                    <span className="text-[var(--text-muted)] shrink-0 tabular-nums">
                      {entry.timestamp}
                    </span>

                    {/* Separator */}
                    <span className="text-[var(--border-default)]">|</span>

                    {/* Level badge */}
                    <span className={`shrink-0 px-1.5 py-0 rounded text-[10px] font-semibold uppercase ${getLevelBadgeStyle(entry.level)}`}>
                      {entry.level.padEnd(5)}
                    </span>

                    {/* Separator */}
                    <span className="text-[var(--border-default)]">|</span>

                    {/* Message */}
                    <span className="text-[var(--text-primary)] break-all">
                      {entry.message}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Auto-scroll indicator */}
          {!autoScroll && filteredEntries.length > 0 && (
            <button
              onClick={() => {
                setAutoScroll(true)
                if (logsContainerRef.current) {
                  logsContainerRef.current.scrollTop = logsContainerRef.current.scrollHeight
                }
              }}
              className="absolute bottom-2 right-4 px-2 py-1 rounded bg-[var(--accent-primary)] text-[var(--primary-foreground)] text-xs font-mono shadow-lg hover:opacity-90 transition-opacity z-20"
            >
              SCROLL TO BOTTOM
            </button>
          )}
        </CardContent>

        {/* Status bar */}
        <div className="px-3 py-1.5 bg-[var(--bg-panel)] border-t border-[var(--border-subtle)] flex items-center justify-between text-[10px] font-mono text-[var(--text-muted)]">
          <span>{filteredEntries.length} entries {activeFilter !== 'ALL' && `(filtered: ${activeFilter})`}</span>
          <span className="flex items-center gap-2">
            <span className={autoScroll ? 'text-[var(--status-online)]' : ''}>
              {autoScroll ? 'AUTO-SCROLL ON' : 'AUTO-SCROLL OFF'}
            </span>
            <span>|</span>
            <span>{activeLogType.toUpperCase()}</span>
          </span>
        </div>
      </Card>
    </div>
  )
}
