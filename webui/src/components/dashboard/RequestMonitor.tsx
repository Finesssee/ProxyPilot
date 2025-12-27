import React, { useState, useEffect, useCallback, useRef } from 'react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { Card, CardHeader, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
    Activity,
    RefreshCw,
    Circle,
    ChevronDown,
    ChevronRight,
    Clock,
    Database,
    AlertCircle,
    CheckCircle2,
    XCircle
} from 'lucide-react'

interface RequestLogEntry {
    id: string
    timestamp: string
    method: string
    path: string
    model: string
    provider: string
    status: number
    latencyMs: number
    inputTokens: number
    outputTokens: number
    error?: string
}

export function RequestMonitor() {
    const { mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()
    const [requests, setRequests] = useState<RequestLogEntry[]>([])
    const [isLive, setIsLive] = useState(false)
    const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())
    const liveIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
    const isRunning = status?.running ?? false

    const fetchRequests = useCallback(async () => {
        try {
            let data: RequestLogEntry[] = []
            if (window.pp_get_requests) {
                data = await window.pp_get_requests()
            } else {
                const res = await mgmtFetch('/v0/management/requests')
                data = res.requests || []
            }
            // Sort by timestamp descending
            setRequests([...data].sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()))
        } catch (e) {
            if (!(e instanceof EngineOfflineError)) {
                showToast(e instanceof Error ? e.message : String(e), 'error')
            }
        }
    }, [mgmtFetch, showToast])

    const toggleLive = () => {
        if (isLive) {
            setIsLive(false)
            if (liveIntervalRef.current) {
                clearInterval(liveIntervalRef.current)
                liveIntervalRef.current = null
            }
        } else {
            setIsLive(true)
            fetchRequests()
            liveIntervalRef.current = setInterval(() => {
                fetchRequests()
            }, 2000)
        }
    }

    const toggleRow = (id: string) => {
        const newExpanded = new Set(expandedRows)
        if (newExpanded.has(id)) {
            newExpanded.delete(id)
        } else {
            newExpanded.add(id)
        }
        setExpandedRows(newExpanded)
    }

    useEffect(() => {
        fetchRequests()
        return () => {
            if (liveIntervalRef.current) {
                clearInterval(liveIntervalRef.current)
            }
        }
    }, [fetchRequests])

    const getStatusColor = (code: number) => {
        if (code >= 200 && code < 300) return 'text-[var(--status-online)]'
        if (code >= 400 && code < 500) return 'text-[var(--status-warning)]'
        if (code >= 500) return 'text-[var(--status-offline)]'
        return 'text-[var(--text-muted)]'
    }

    const getStatusIcon = (code: number) => {
        if (code >= 200 && code < 300) return <CheckCircle2 className="h-3 w-3" />
        if (code >= 400 && code < 500) return <AlertCircle className="h-3 w-3" />
        if (code >= 500) return <XCircle className="h-3 w-3" />
        return <Circle className="h-3 w-3" />
    }

    const formatTime = (ts: string) => {
        const date = new Date(ts)
        return date.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
    }

    return (
        <Card className="backdrop-blur-sm bg-[var(--bg-void)] border-[var(--border-default)] shadow-xl overflow-hidden">
            <CardHeader className="pb-3 border-b border-[var(--border-subtle)] bg-[var(--bg-panel)]">
                <div className="flex items-center justify-between flex-wrap gap-3">
                    <div className="flex items-center gap-3">
                        <div className="flex items-center gap-2">
                            <Activity className={`h-4 w-4 ${isLive && isRunning ? 'text-[var(--accent-glow)] animate-pulse' : 'text-[var(--text-muted)]'}`} />
                            <span className="font-mono text-sm font-semibold tracking-wider text-[var(--text-primary)]">
                                REQUEST MONITOR
                            </span>
                        </div>
                        {isMgmtLoading && <RefreshCw className="h-3 w-3 animate-spin text-muted-foreground" />}

                        <button
                            onClick={toggleLive}
                            disabled={!isRunning}
                            className={`flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-mono transition-all ${isLive && isRunning
                                ? 'bg-[var(--accent-glow)]/20 text-[var(--accent-glow)]'
                                : 'bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
                                } ${!isRunning ? 'opacity-50 cursor-not-allowed' : ''}`}
                        >
                            <span className={`h-1.5 w-1.5 rounded-full ${isLive && isRunning ? 'bg-[var(--accent-glow)]' : 'bg-[var(--text-muted)]'}`} />
                            LIVE
                        </button>
                    </div>

                    <div className="flex items-center gap-2">
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={fetchRequests}
                            disabled={!isRunning || isMgmtLoading}
                            className="text-xs h-7 font-mono gap-1"
                        >
                            <RefreshCw className="h-3 w-3" />
                            REFRESH
                        </Button>
                    </div>
                </div>
            </CardHeader>

            <CardContent className="p-0">
                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse font-mono text-xs">
                        <thead>
                            <tr className="bg-[var(--bg-panel)]/50 text-[var(--text-muted)] border-b border-[var(--border-subtle)]">
                                <th className="px-4 py-2 font-medium">TIME</th>
                                <th className="px-4 py-2 font-medium">MODEL</th>
                                <th className="px-4 py-2 font-medium">PROVIDER</th>
                                <th className="px-4 py-2 font-medium">STATUS</th>
                                <th className="px-4 py-2 font-medium">LATENCY</th>
                                <th className="px-4 py-2 font-medium">TOKENS</th>
                                <th className="px-4 py-2 font-medium w-10"></th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-[var(--border-subtle)]/30">
                            {!isRunning ? (
                                <tr>
                                    <td colSpan={7} className="p-8 text-center text-[var(--text-muted)]">
                                        <p className="text-xs uppercase tracking-widest">⚠️ Engine Offline</p>
                                    </td>
                                </tr>
                            ) : requests.length === 0 ? (
                                <tr>
                                    <td colSpan={7} className="p-8 text-center text-[var(--text-muted)]">
                                        No requests recorded yet.
                                    </td>
                                </tr>
                            ) : (
                                requests.map((req) => (
                                    <React.Fragment key={req.id}>
                                        <tr
                                            className="hover:bg-[var(--bg-panel)]/50 transition-colors cursor-pointer group"
                                            onClick={() => toggleRow(req.id)}
                                        >
                                            <td className="px-4 py-2.5 text-[var(--text-muted)] tabular-nums">
                                                {formatTime(req.timestamp)}
                                            </td>
                                            <td className="px-4 py-2.5 text-[var(--text-primary)] font-medium">
                                                {req.model || '-'}
                                            </td>
                                            <td className="px-4 py-2.5">
                                                <span className="px-1.5 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-secondary)] text-[10px]">
                                                    {req.provider?.toUpperCase() || 'UNKNOWN'}
                                                </span>
                                            </td>
                                            <td className={`px-4 py-2.5 font-bold ${getStatusColor(req.status)}`}>
                                                <div className="flex items-center gap-1.5">
                                                    {getStatusIcon(req.status)}
                                                    {req.status}
                                                </div>
                                            </td>
                                            <td className="px-4 py-2.5 text-[var(--text-secondary)] tabular-nums">
                                                {req.latencyMs}ms
                                            </td>
                                            <td className="px-4 py-2.5 text-[var(--text-muted)] tabular-nums">
                                                <div className="flex flex-col">
                                                    <span>In: {req.inputTokens}</span>
                                                    <span>Out: {req.outputTokens}</span>
                                                </div>
                                            </td>
                                            <td className="px-4 py-2.5 text-right">
                                                {expandedRows.has(req.id) ? (
                                                    <ChevronDown className="h-4 w-4 text-[var(--text-muted)]" />
                                                ) : (
                                                    <ChevronRight className="h-4 w-4 text-[var(--text-muted)] group-hover:text-[var(--text-primary)]" />
                                                )}
                                            </td>
                                        </tr>
                                        {expandedRows.has(req.id) && (
                                            <tr className="bg-[var(--bg-panel)]/30">
                                                <td colSpan={7} className="px-4 py-4 border-l-2 border-l-[var(--accent-glow)]">
                                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                        <div className="space-y-2">
                                                            <div className="flex items-center gap-2 text-[var(--text-muted)] text-[10px] uppercase tracking-wider">
                                                                <Clock className="h-3 w-3" />
                                                                Request Details
                                                            </div>
                                                            <div className="space-y-1 text-[var(--text-secondary)]">
                                                                <div className="flex justify-between">
                                                                    <span>Method:</span>
                                                                    <span className="text-[var(--text-primary)]">{req.method}</span>
                                                                </div>
                                                                <div className="flex justify-between">
                                                                    <span>Path:</span>
                                                                    <span className="text-[var(--text-primary)] break-all ml-4">{req.path}</span>
                                                                </div>
                                                                <div className="flex justify-between">
                                                                    <span>ID:</span>
                                                                    <span className="text-[var(--text-primary)] text-[10px]">{req.id}</span>
                                                                </div>
                                                            </div>
                                                        </div>
                                                        <div className="space-y-2">
                                                            <div className="flex items-center gap-2 text-[var(--text-muted)] text-[10px] uppercase tracking-wider">
                                                                <Database className="h-3 w-3" />
                                                                Performance & Usage
                                                            </div>
                                                            <div className="space-y-1 text-[var(--text-secondary)]">
                                                                <div className="flex justify-between">
                                                                    <span>Latency:</span>
                                                                    <span className="text-[var(--text-primary)]">{req.latencyMs}ms</span>
                                                                </div>
                                                                <div className="flex justify-between">
                                                                    <span>Total Tokens:</span>
                                                                    <span className="text-[var(--text-primary)]">{req.inputTokens + req.outputTokens}</span>
                                                                </div>
                                                                {req.error && (
                                                                    <div className="mt-2 p-2 rounded bg-[var(--status-offline)]/10 border border-[var(--status-offline)]/20 text-[var(--status-offline)]">
                                                                        <div className="flex items-center gap-1 mb-1 font-bold">
                                                                            <AlertCircle className="h-3 w-3" />
                                                                            ERROR
                                                                        </div>
                                                                        <div className="text-[10px] break-all">{req.error}</div>
                                                                    </div>
                                                                )}
                                                            </div>
                                                        </div>
                                                    </div>
                                                </td>
                                            </tr>
                                        )}
                                    </React.Fragment>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </CardContent>

            <div className="px-3 py-1.5 bg-[var(--bg-panel)] border-t border-[var(--border-subtle)] flex items-center justify-between text-[10px] font-mono text-[var(--text-muted)]">
                <span>{requests.length} requests tracked</span>
                <span className="flex items-center gap-2">
                    <span className={isLive ? 'text-[var(--accent-glow)]' : ''}>
                        {isLive ? 'LIVE POLLING ON' : 'LIVE POLLING OFF'}
                    </span>
                    <span>|</span>
                    <span>2s INTERVAL</span>
                </span>
            </div>
        </Card>
    )
}
