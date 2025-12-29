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
    XCircle,
    Download,
    Trash2,
    Filter,
    History,
    Radio,
    ChevronLeft,
    ChevronsLeft,
    ChevronsRight,
    Save
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

interface HistoryStats {
    total_requests: number
    success_count: number
    failure_count: number
    total_tokens_in: number
    total_tokens_out: number
    total_cost_usd: number
    direct_api_cost: number
    savings: number
    savings_percent: number
}

type ViewMode = 'live' | 'history'

export function RequestMonitor() {
    const { mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()
    const [requests, setRequests] = useState<RequestLogEntry[]>([])
    const [historyStats, setHistoryStats] = useState<HistoryStats | null>(null)
    const [isLive, setIsLive] = useState(false)
    const [viewMode, setViewMode] = useState<ViewMode>('live')
    const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())
    const liveIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
    const isRunning = status?.running ?? false

    // Filter state
    const [showFilters, setShowFilters] = useState(false)
    const [filterModel, setFilterModel] = useState('')
    const [filterProvider, setFilterProvider] = useState('')
    const [filterStatus, setFilterStatus] = useState<'all' | 'success' | 'error'>('all')
    const [filterStartDate, setFilterStartDate] = useState('')
    const [filterEndDate, setFilterEndDate] = useState('')

    // Pagination state
    const [page, setPage] = useState(1)
    const [pageSize] = useState(50)
    const [totalCount, setTotalCount] = useState(0)

    // Unique values for filters
    const [availableModels, setAvailableModels] = useState<string[]>([])
    const [availableProviders, setAvailableProviders] = useState<string[]>([])

    const fetchLiveRequests = useCallback(async () => {
        try {
            let data: RequestLogEntry[] = []
            if (window.pp_get_requests) {
                data = await window.pp_get_requests()
            } else {
                const res = await mgmtFetch('/v0/management/requests')
                data = res.requests || []
            }
            setRequests([...data].sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()))

            // Extract unique models and providers
            const models = new Set<string>()
            const providers = new Set<string>()
            data.forEach(r => {
                if (r.model) models.add(r.model)
                if (r.provider) providers.add(r.provider)
            })
            setAvailableModels(Array.from(models).sort())
            setAvailableProviders(Array.from(providers).sort())
        } catch (e) {
            if (!(e instanceof EngineOfflineError)) {
                showToast(e instanceof Error ? e.message : String(e), 'error')
            }
        }
    }, [mgmtFetch, showToast])

    const fetchHistory = useCallback(async () => {
        try {
            const params = new URLSearchParams()
            params.set('limit', String(pageSize))
            params.set('offset', String((page - 1) * pageSize))

            if (filterModel) params.set('model', filterModel)
            if (filterProvider) params.set('provider', filterProvider)
            if (filterStartDate) params.set('start_date', new Date(filterStartDate).toISOString())
            if (filterEndDate) params.set('end_date', new Date(filterEndDate).toISOString())
            if (filterStatus === 'success') {
                params.set('status_min', '200')
                params.set('status_max', '299')
            } else if (filterStatus === 'error') {
                params.set('errors_only', 'true')
            }

            const res = await mgmtFetch(`/v0/management/request-history?${params.toString()}`)
            setRequests(res.entries || [])
            setTotalCount(res.total || 0)

            // Also fetch stats
            const statsRes = await mgmtFetch('/v0/management/request-history/stats')
            setHistoryStats(statsRes.stats || null)

            // Extract unique models and providers from stats
            if (statsRes.stats?.by_model) {
                setAvailableModels(Object.keys(statsRes.stats.by_model).sort())
            }
            if (statsRes.stats?.by_provider) {
                setAvailableProviders(Object.keys(statsRes.stats.by_provider).sort())
            }
        } catch (e) {
            if (!(e instanceof EngineOfflineError)) {
                showToast(e instanceof Error ? e.message : String(e), 'error')
            }
        }
    }, [mgmtFetch, showToast, page, pageSize, filterModel, filterProvider, filterStatus, filterStartDate, filterEndDate])

    const toggleLive = () => {
        if (isLive) {
            setIsLive(false)
            if (liveIntervalRef.current) {
                clearInterval(liveIntervalRef.current)
                liveIntervalRef.current = null
            }
        } else {
            setIsLive(true)
            fetchLiveRequests()
            liveIntervalRef.current = setInterval(() => {
                fetchLiveRequests()
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

    const handleExport = async () => {
        try {
            const res = await mgmtFetch('/v0/management/request-history/export')
            const blob = new Blob([JSON.stringify(res, null, 2)], { type: 'application/json' })
            const url = URL.createObjectURL(blob)
            const a = document.createElement('a')
            a.href = url
            a.download = `proxypilot-request-history-${new Date().toISOString().split('T')[0]}.json`
            a.click()
            URL.revokeObjectURL(url)
            showToast('Request history exported', 'success')
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        }
    }

    const handleClear = async () => {
        if (!confirm('Are you sure you want to clear all request history? This cannot be undone.')) {
            return
        }
        try {
            await mgmtFetch('/v0/management/request-history', { method: 'DELETE' })
            showToast('Request history cleared', 'success')
            setRequests([])
            setTotalCount(0)
            setHistoryStats(null)
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        }
    }

    const handleSave = async () => {
        try {
            await mgmtFetch('/v0/management/request-history/save', { method: 'POST' })
            showToast('Request history saved to disk', 'success')
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        }
    }

    useEffect(() => {
        if (viewMode === 'live') {
            fetchLiveRequests()
        } else {
            fetchHistory()
        }
        return () => {
            if (liveIntervalRef.current) {
                clearInterval(liveIntervalRef.current)
            }
        }
    }, [viewMode, fetchLiveRequests, fetchHistory])

    // Reset page when filters change
    useEffect(() => {
        setPage(1)
    }, [filterModel, filterProvider, filterStatus, filterStartDate, filterEndDate])

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

    const formatDate = (ts: string) => {
        const date = new Date(ts)
        return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
    }

    const totalPages = Math.ceil(totalCount / pageSize)

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

                        {/* View Mode Toggle */}
                        <div className="flex items-center gap-1 bg-[var(--bg-elevated)] rounded p-0.5">
                            <button
                                onClick={() => { setViewMode('live'); setIsLive(false) }}
                                className={`flex items-center gap-1.5 px-2 py-1 rounded text-xs font-mono transition-all ${viewMode === 'live'
                                    ? 'bg-[var(--accent-primary)]/20 text-[var(--accent-primary)]'
                                    : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
                                    }`}
                            >
                                <Radio className="h-3 w-3" />
                                LIVE
                            </button>
                            <button
                                onClick={() => { setViewMode('history'); setIsLive(false); if (liveIntervalRef.current) { clearInterval(liveIntervalRef.current); liveIntervalRef.current = null } }}
                                className={`flex items-center gap-1.5 px-2 py-1 rounded text-xs font-mono transition-all ${viewMode === 'history'
                                    ? 'bg-[var(--accent-primary)]/20 text-[var(--accent-primary)]'
                                    : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
                                    }`}
                            >
                                <History className="h-3 w-3" />
                                HISTORY
                            </button>
                        </div>

                        {viewMode === 'live' && (
                            <button
                                onClick={toggleLive}
                                disabled={!isRunning}
                                className={`flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-mono transition-all ${isLive && isRunning
                                    ? 'bg-[var(--accent-glow)]/20 text-[var(--accent-glow)]'
                                    : 'bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
                                    } ${!isRunning ? 'opacity-50 cursor-not-allowed' : ''}`}
                            >
                                <span className={`h-1.5 w-1.5 rounded-full ${isLive && isRunning ? 'bg-[var(--accent-glow)]' : 'bg-[var(--text-muted)]'}`} />
                                AUTO
                            </button>
                        )}
                    </div>

                    <div className="flex items-center gap-2">
                        {viewMode === 'history' && (
                            <>
                                <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={() => setShowFilters(!showFilters)}
                                    className={`text-xs h-7 font-mono gap-1 ${showFilters ? 'bg-[var(--accent-primary)]/10 border-[var(--accent-primary)]' : ''}`}
                                >
                                    <Filter className="h-3 w-3" />
                                    FILTER
                                </Button>
                                <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={handleSave}
                                    disabled={!isRunning || isMgmtLoading}
                                    className="text-xs h-7 font-mono gap-1"
                                >
                                    <Save className="h-3 w-3" />
                                    SAVE
                                </Button>
                                <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={handleExport}
                                    disabled={!isRunning || isMgmtLoading}
                                    className="text-xs h-7 font-mono gap-1"
                                >
                                    <Download className="h-3 w-3" />
                                    EXPORT
                                </Button>
                                <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={handleClear}
                                    disabled={!isRunning || isMgmtLoading}
                                    className="text-xs h-7 font-mono gap-1 text-[var(--status-offline)] hover:bg-[var(--status-offline)]/10"
                                >
                                    <Trash2 className="h-3 w-3" />
                                    CLEAR
                                </Button>
                            </>
                        )}
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={viewMode === 'live' ? fetchLiveRequests : fetchHistory}
                            disabled={!isRunning || isMgmtLoading}
                            className="text-xs h-7 font-mono gap-1"
                        >
                            <RefreshCw className="h-3 w-3" />
                            REFRESH
                        </Button>
                    </div>
                </div>

                {/* Filter Panel */}
                {showFilters && viewMode === 'history' && (
                    <div className="mt-3 pt-3 border-t border-[var(--border-subtle)] grid grid-cols-2 md:grid-cols-5 gap-3">
                        <div>
                            <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase mb-1">Model</label>
                            <select
                                value={filterModel}
                                onChange={(e) => setFilterModel(e.target.value)}
                                className="w-full px-2 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-xs font-mono"
                            >
                                <option value="">All Models</option>
                                {availableModels.map(m => <option key={m} value={m}>{m}</option>)}
                            </select>
                        </div>
                        <div>
                            <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase mb-1">Provider</label>
                            <select
                                value={filterProvider}
                                onChange={(e) => setFilterProvider(e.target.value)}
                                className="w-full px-2 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-xs font-mono"
                            >
                                <option value="">All Providers</option>
                                {availableProviders.map(p => <option key={p} value={p}>{p}</option>)}
                            </select>
                        </div>
                        <div>
                            <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase mb-1">Status</label>
                            <select
                                value={filterStatus}
                                onChange={(e) => setFilterStatus(e.target.value as 'all' | 'success' | 'error')}
                                className="w-full px-2 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-xs font-mono"
                            >
                                <option value="all">All</option>
                                <option value="success">Success (2xx)</option>
                                <option value="error">Errors</option>
                            </select>
                        </div>
                        <div>
                            <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase mb-1">From</label>
                            <input
                                type="date"
                                value={filterStartDate}
                                onChange={(e) => setFilterStartDate(e.target.value)}
                                className="w-full px-2 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-xs font-mono"
                            />
                        </div>
                        <div>
                            <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase mb-1">To</label>
                            <input
                                type="date"
                                value={filterEndDate}
                                onChange={(e) => setFilterEndDate(e.target.value)}
                                className="w-full px-2 py-1.5 rounded border border-[var(--border-default)] bg-[var(--bg-elevated)] text-xs font-mono"
                            />
                        </div>
                    </div>
                )}

                {/* History Stats */}
                {viewMode === 'history' && historyStats && (
                    <div className="mt-3 pt-3 border-t border-[var(--border-subtle)] grid grid-cols-3 md:grid-cols-6 gap-4 text-center">
                        <div>
                            <div className="text-lg font-bold text-[var(--text-primary)]">{historyStats.total_requests}</div>
                            <div className="text-[10px] font-mono text-[var(--text-muted)] uppercase">Total</div>
                        </div>
                        <div>
                            <div className="text-lg font-bold text-[var(--status-online)]">{historyStats.success_count}</div>
                            <div className="text-[10px] font-mono text-[var(--text-muted)] uppercase">Success</div>
                        </div>
                        <div>
                            <div className="text-lg font-bold text-[var(--status-offline)]">{historyStats.failure_count}</div>
                            <div className="text-[10px] font-mono text-[var(--text-muted)] uppercase">Failed</div>
                        </div>
                        <div>
                            <div className="text-lg font-bold text-[var(--text-primary)]">{(historyStats.total_tokens_in + historyStats.total_tokens_out).toLocaleString()}</div>
                            <div className="text-[10px] font-mono text-[var(--text-muted)] uppercase">Tokens</div>
                        </div>
                        <div>
                            <div className="text-lg font-bold text-emerald-400">${historyStats.total_cost_usd.toFixed(2)}</div>
                            <div className="text-[10px] font-mono text-[var(--text-muted)] uppercase">Cost</div>
                        </div>
                        <div>
                            <div className="text-lg font-bold text-emerald-400">${historyStats.savings.toFixed(2)}</div>
                            <div className="text-[10px] font-mono text-[var(--text-muted)] uppercase">Saved</div>
                        </div>
                    </div>
                )}
            </CardHeader>

            <CardContent className="p-0">
                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse font-mono text-xs">
                        <thead>
                            <tr className="bg-[var(--bg-panel)]/50 text-[var(--text-muted)] border-b border-[var(--border-subtle)]">
                                <th className="px-4 py-2 font-medium">{viewMode === 'history' ? 'DATE' : 'TIME'}</th>
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
                                        <p className="text-xs uppercase tracking-widest">Engine Offline</p>
                                    </td>
                                </tr>
                            ) : requests.length === 0 ? (
                                <tr>
                                    <td colSpan={7} className="p-8 text-center text-[var(--text-muted)]">
                                        {viewMode === 'history' ? 'No request history found.' : 'No requests recorded yet.'}
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
                                                {viewMode === 'history' ? (
                                                    <div className="flex flex-col">
                                                        <span>{formatDate(req.timestamp)}</span>
                                                        <span className="text-[10px]">{formatTime(req.timestamp)}</span>
                                                    </div>
                                                ) : formatTime(req.timestamp)}
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
                {viewMode === 'live' ? (
                    <>
                        <span>{requests.length} requests tracked</span>
                        <span className="flex items-center gap-2">
                            <span className={isLive ? 'text-[var(--accent-glow)]' : ''}>
                                {isLive ? 'AUTO REFRESH ON' : 'AUTO REFRESH OFF'}
                            </span>
                            <span>|</span>
                            <span>2s INTERVAL</span>
                        </span>
                    </>
                ) : (
                    <>
                        <span>Showing {requests.length} of {totalCount} entries</span>
                        {totalPages > 1 && (
                            <div className="flex items-center gap-1">
                                <button
                                    onClick={() => setPage(1)}
                                    disabled={page === 1}
                                    className="p-1 rounded hover:bg-[var(--bg-elevated)] disabled:opacity-30"
                                >
                                    <ChevronsLeft className="h-3 w-3" />
                                </button>
                                <button
                                    onClick={() => setPage(p => Math.max(1, p - 1))}
                                    disabled={page === 1}
                                    className="p-1 rounded hover:bg-[var(--bg-elevated)] disabled:opacity-30"
                                >
                                    <ChevronLeft className="h-3 w-3" />
                                </button>
                                <span className="px-2">
                                    Page {page} of {totalPages}
                                </span>
                                <button
                                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                                    disabled={page === totalPages}
                                    className="p-1 rounded hover:bg-[var(--bg-elevated)] disabled:opacity-30"
                                >
                                    <ChevronRight className="h-3 w-3" />
                                </button>
                                <button
                                    onClick={() => setPage(totalPages)}
                                    disabled={page === totalPages}
                                    className="p-1 rounded hover:bg-[var(--bg-elevated)] disabled:opacity-30"
                                >
                                    <ChevronsRight className="h-3 w-3" />
                                </button>
                            </div>
                        )}
                    </>
                )}
            </div>
        </Card>
    )
}
