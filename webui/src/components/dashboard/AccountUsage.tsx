import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useProxyContext } from '@/hooks/useProxyContext'
import { Loader2, Users, Zap, Activity, RefreshCw, Clock } from 'lucide-react'
import { cn } from '@/lib/utils'

interface AccountUsageData {
    total_input_tokens: number
    total_output_tokens: number
    request_count: number
    daily_input_tokens: number
    daily_output_tokens: number
    daily_request_count: number
    day_started_at?: string
}

interface AuthFileInfo {
    id: string
    name: string
    provider?: string
    type?: string
    email?: string
    label?: string
    status?: string
    usage?: AccountUsageData
}

// Provider color mapping
const providerColors: Record<string, string> = {
    claude: 'oklch(0.60 0.15 35)',
    anthropic: 'oklch(0.60 0.15 35)',
    gemini: 'oklch(0.55 0.18 250)',
    'gemini-cli': 'oklch(0.58 0.16 240)',
    codex: 'oklch(0.60 0.16 145)',
    qwen: 'oklch(0.60 0.14 280)',
    antigravity: 'oklch(0.65 0.20 320)',
    kiro: 'oklch(0.58 0.17 200)',
    minimax: 'oklch(0.60 0.18 60)',
    zhipu: 'oklch(0.55 0.15 180)',
}

function getProviderColor(provider: string): string {
    const p = provider?.toLowerCase() || ''
    for (const [key, color] of Object.entries(providerColors)) {
        if (p.includes(key)) return color
    }
    return 'oklch(0.55 0.15 220)' // default blue
}

function formatTokens(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
    if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
    return n.toString()
}

function formatTimeAgo(dateStr?: string): string {
    if (!dateStr) return 'N/A'
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
    const diffMins = Math.floor(diffMs / (1000 * 60))

    if (diffHours >= 24) {
        const days = Math.floor(diffHours / 24)
        return `${days}d ago`
    }
    if (diffHours > 0) return `${diffHours}h ago`
    if (diffMins > 0) return `${diffMins}m ago`
    return 'just now'
}

interface AccountCardProps {
    account: AuthFileInfo
    maxTotalTokens: number
}

function AccountCard({ account, maxTotalTokens }: AccountCardProps) {
    const usage = account.usage
    const totalTokens = (usage?.total_input_tokens || 0) + (usage?.total_output_tokens || 0)
    const dailyTokens = (usage?.daily_input_tokens || 0) + (usage?.daily_output_tokens || 0)
    const color = getProviderColor(account.provider || account.type || '')
    const barWidth = maxTotalTokens > 0 ? (totalTokens / maxTotalTokens) * 100 : 0

    const displayName = account.email || account.label || account.name || account.id
    const providerName = account.provider || account.type || 'unknown'

    return (
        <div
            className={cn(
                'relative p-4 rounded-lg border border-[var(--border-subtle)]',
                'bg-[var(--bg-panel)] hover:border-transparent transition-all duration-200',
                'group'
            )}
            style={{ ['--provider-color' as string]: color }}
        >
            {/* Hover glow */}
            <div
                className="absolute inset-0 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
                style={{
                    boxShadow: `0 0 20px color-mix(in oklch, ${color} 20%, transparent), inset 0 0 15px color-mix(in oklch, ${color} 5%, transparent)`,
                }}
            />
            {/* Hover border */}
            <div
                className="absolute inset-0 rounded-lg border-2 opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
                style={{ borderColor: color }}
            />

            {/* Header */}
            <div className="relative flex items-start justify-between mb-3">
                <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                        <span
                            className="inline-flex px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider"
                            style={{
                                background: `color-mix(in oklch, ${color} 20%, transparent)`,
                                color: color,
                            }}
                        >
                            {providerName}
                        </span>
                        {account.status && (
                            <span
                                className={cn(
                                    'inline-flex px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider',
                                    account.status === 'active' || account.status === 'ok'
                                        ? 'bg-[var(--status-online)]/15 text-[var(--status-online)]'
                                        : 'bg-[var(--text-muted)]/15 text-[var(--text-muted)]'
                                )}
                            >
                                {account.status}
                            </span>
                        )}
                    </div>
                    <p className="text-sm font-medium text-[var(--text-primary)] truncate" title={displayName}>
                        {displayName}
                    </p>
                </div>
            </div>

            {/* Stats Grid */}
            <div className="relative grid grid-cols-2 gap-3 mb-3">
                <div className="space-y-1">
                    <div className="flex items-center gap-1 text-[10px] uppercase tracking-wider text-[var(--text-muted)]">
                        <Activity className="h-3 w-3" />
                        Requests
                    </div>
                    <div className="text-lg font-bold text-[var(--text-primary)]">
                        {usage?.request_count?.toLocaleString() || 0}
                    </div>
                </div>
                <div className="space-y-1">
                    <div className="flex items-center gap-1 text-[10px] uppercase tracking-wider text-[var(--text-muted)]">
                        <Zap className="h-3 w-3" />
                        Total Tokens
                    </div>
                    <div className="text-lg font-bold text-[var(--text-primary)]">
                        {formatTokens(totalTokens)}
                    </div>
                </div>
            </div>

            {/* Token Bar */}
            <div className="relative mb-3">
                <div className="flex justify-between text-[10px] text-[var(--text-muted)] mb-1">
                    <span>In: {formatTokens(usage?.total_input_tokens || 0)}</span>
                    <span>Out: {formatTokens(usage?.total_output_tokens || 0)}</span>
                </div>
                <div className="h-2 w-full rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                    <div
                        className="h-full rounded-full transition-all duration-500"
                        style={{
                            width: `${Math.max(barWidth, 2)}%`,
                            background: `linear-gradient(90deg, ${color}, color-mix(in oklch, ${color} 70%, white))`,
                            boxShadow: `0 0 8px ${color}`,
                        }}
                    />
                </div>
            </div>

            {/* Daily Stats */}
            <div className="relative border-t border-[var(--border-subtle)] pt-3">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)]">
                        <Clock className="h-3 w-3" />
                        Daily: {usage?.daily_request_count || 0} reqs, {formatTokens(dailyTokens)} tokens
                    </div>
                    <div className="text-[10px] text-[var(--text-muted)]">
                        Reset {formatTimeAgo(usage?.day_started_at)}
                    </div>
                </div>
            </div>
        </div>
    )
}

export function AccountUsage() {
    const { mgmtFetch, status } = useProxyContext()
    const [accounts, setAccounts] = useState<AuthFileInfo[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const isRunning = status?.running ?? false

    const fetchData = async () => {
        if (!isRunning) {
            setAccounts([])
            setLoading(false)
            return
        }
        try {
            const res = await mgmtFetch('/v0/management/auth-files')
            const files = res.files || []
            setAccounts(files)
            setError(null)
        } catch (e) {
            console.error('Failed to fetch auth files:', e)
            setError(e instanceof Error ? e.message : String(e))
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchData()
        const interval = setInterval(fetchData, 30000)
        return () => clearInterval(interval)
    }, [isRunning])

    // Calculate max tokens for relative bar sizing
    const maxTotalTokens = Math.max(
        ...accounts.map(a => (a.usage?.total_input_tokens || 0) + (a.usage?.total_output_tokens || 0)),
        1
    )

    // Aggregate totals
    const totals = accounts.reduce(
        (acc, a) => ({
            requests: acc.requests + (a.usage?.request_count || 0),
            inputTokens: acc.inputTokens + (a.usage?.total_input_tokens || 0),
            outputTokens: acc.outputTokens + (a.usage?.total_output_tokens || 0),
            dailyRequests: acc.dailyRequests + (a.usage?.daily_request_count || 0),
        }),
        { requests: 0, inputTokens: 0, outputTokens: 0, dailyRequests: 0 }
    )

    if (loading && accounts.length === 0) {
        return (
            <div className="flex h-64 items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        )
    }

    if (!isRunning) {
        return (
            <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                <CardHeader>
                    <div className="flex items-center gap-2">
                        <Users className="h-4 w-4 text-muted-foreground" />
                        <CardTitle className="text-lg">Account Usage</CardTitle>
                    </div>
                    <CardDescription>Per-account usage statistics</CardDescription>
                </CardHeader>
                <CardContent>
                    <div className="p-4 rounded-lg border border-yellow-500/30 bg-yellow-500/5 text-yellow-500 text-xs text-center uppercase tracking-widest font-mono">
                        Start the proxy engine to view account usage
                    </div>
                </CardContent>
            </Card>
        )
    }

    if (error && accounts.length === 0) {
        return (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
                <p className="text-red-300">Failed to load account usage</p>
                <p className="mt-2 text-xs text-red-400/70">{error}</p>
            </div>
        )
    }

    if (accounts.length === 0) {
        return (
            <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                <CardHeader>
                    <div className="flex items-center gap-2">
                        <Users className="h-4 w-4 text-muted-foreground" />
                        <CardTitle className="text-lg">Account Usage</CardTitle>
                    </div>
                    <CardDescription>Per-account usage statistics</CardDescription>
                </CardHeader>
                <CardContent>
                    <div className="py-8 text-center text-sm text-muted-foreground">
                        No accounts configured. Add providers in the Providers tab.
                    </div>
                </CardContent>
            </Card>
        )
    }

    return (
        <div className="space-y-6">
            {/* Summary Card */}
            <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <Users className="h-4 w-4 text-muted-foreground" />
                            <CardTitle className="text-lg">Account Usage</CardTitle>
                        </div>
                        <button
                            onClick={() => { setLoading(true); fetchData() }}
                            className="p-1.5 rounded-md hover:bg-[var(--bg-elevated)] transition-colors"
                            title="Refresh"
                        >
                            <RefreshCw className={cn('h-4 w-4 text-[var(--text-muted)]', loading && 'animate-spin')} />
                        </button>
                    </div>
                    <CardDescription>
                        {accounts.length} account{accounts.length !== 1 ? 's' : ''} across all providers
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <div className="grid gap-4 md:grid-cols-4">
                        <div className="space-y-1">
                            <p className="text-xs text-[var(--text-muted)] uppercase tracking-wider">Total Requests</p>
                            <p className="text-2xl font-bold text-[var(--text-primary)]">
                                {totals.requests.toLocaleString()}
                            </p>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs text-[var(--text-muted)] uppercase tracking-wider">Input Tokens</p>
                            <p className="text-2xl font-bold text-[var(--text-primary)]">
                                {formatTokens(totals.inputTokens)}
                            </p>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs text-[var(--text-muted)] uppercase tracking-wider">Output Tokens</p>
                            <p className="text-2xl font-bold text-[var(--text-primary)]">
                                {formatTokens(totals.outputTokens)}
                            </p>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs text-[var(--text-muted)] uppercase tracking-wider">Today's Requests</p>
                            <p className="text-2xl font-bold text-[var(--text-primary)]">
                                {totals.dailyRequests.toLocaleString()}
                            </p>
                        </div>
                    </div>
                </CardContent>
            </Card>

            {/* Account Grid */}
            <div
                className="grid gap-4"
                style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))' }}
            >
                {accounts.map((account) => (
                    <AccountCard key={account.id} account={account} maxTotalTokens={maxTotalTokens} />
                ))}
            </div>
        </div>
    )
}
