import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useProxyContext } from '@/hooks/useProxyContext'
import { Loader2, BarChart3, Activity, Zap, DollarSign, PieChart } from 'lucide-react'

interface DailyUsage {
    date: string
    requests: number
    tokens: number
    input_tokens: number
    output_tokens: number
}

interface UsageData {
    total_requests: number
    success_count: number
    failure_count: number
    total_input_tokens: number
    total_output_tokens: number
    estimated_cost_saved: number
    by_model: Record<string, number>
    by_provider: Record<string, number>
    daily: DailyUsage[]
}

export function UsageStats() {
    const { mgmtFetch } = useProxyContext()
    const [data, setData] = useState<UsageData | null>(null)
    const [error, setError] = useState<string | null>(null)
    const [loading, setLoading] = useState(true)

    const fetchData = async () => {
        try {
            let result: any
            if (window.pp_get_usage) {
                result = await window.pp_get_usage()
            } else if (mgmtFetch) {
                result = await mgmtFetch('/v0/management/usage')
            } else {
                throw new Error('No method available to fetch usage data')
            }

            // Handle various response shapes
            const usageData = result?.usage || result
            if (usageData && typeof usageData === 'object') {
                // Ensure all required fields have defaults
                setData({
                    total_requests: usageData.total_requests ?? 0,
                    success_count: usageData.success_count ?? 0,
                    failure_count: usageData.failure_count ?? 0,
                    total_input_tokens: usageData.total_input_tokens ?? 0,
                    total_output_tokens: usageData.total_output_tokens ?? 0,
                    estimated_cost_saved: usageData.estimated_cost_saved ?? 0,
                    by_model: usageData.by_model ?? {},
                    by_provider: usageData.by_provider ?? {},
                    daily: usageData.daily ?? [],
                })
                setError(null)
            }
        } catch (e) {
            console.error('Failed to fetch usage stats:', e)
            setError(e instanceof Error ? e.message : String(e))
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchData()
        const interval = setInterval(fetchData, 30000) // Refresh every 30s
        return () => clearInterval(interval)
    }, [])

    if (loading && !data) {
        return (
            <div className="flex h-64 items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        )
    }

    if (error && !data) {
        return (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center">
                <p className="text-red-300">Failed to load usage statistics</p>
                <p className="mt-2 text-xs text-red-400/70">{error}</p>
            </div>
        )
    }

    if (!data) {
        return (
            <div className="rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-8 text-center">
                <BarChart3 className="mx-auto h-12 w-12 text-[var(--text-muted)] opacity-50" />
                <p className="mt-4 text-[var(--text-primary)]">No usage data available yet</p>
                <p className="mt-2 text-xs text-[var(--text-muted)]">
                    Start making requests through the proxy to see statistics here.
                </p>
            </div>
        )
    }

    const maxDailyRequests = Math.max(...data.daily.map((d) => d.requests), 1)
    const providerEntries = Object.entries(data.by_provider).sort((a, b) => b[1] - a[1])
    const totalProviderRequests = providerEntries.reduce((acc, [_, count]) => acc + count, 0)

    return (
        <div className="space-y-6">
            {/* KPI Cards */}
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
                        <Activity className="h-4 w-4 text-blue-400" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">{data.total_requests.toLocaleString()}</div>
                        <p className="text-xs text-muted-foreground">
                            {data.success_count.toLocaleString()} successful
                        </p>
                    </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Input Tokens</CardTitle>
                        <Zap className="h-4 w-4 text-amber-400" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">{data.total_input_tokens.toLocaleString()}</div>
                        <p className="text-xs text-muted-foreground">Prompt tokens</p>
                    </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Output Tokens</CardTitle>
                        <Zap className="h-4 w-4 text-emerald-400" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">{data.total_output_tokens.toLocaleString()}</div>
                        <p className="text-xs text-muted-foreground">Completion tokens</p>
                    </CardContent>
                </Card>

                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                        <CardTitle className="text-sm font-medium">Est. Savings</CardTitle>
                        <DollarSign className="h-4 w-4 text-purple-400" />
                    </CardHeader>
                    <CardContent>
                        <div className="text-2xl font-bold">${data.estimated_cost_saved.toFixed(2)}</div>
                        <p className="text-xs text-muted-foreground">Based on public rates</p>
                    </CardContent>
                </Card>
            </div>

            <div className="grid gap-6 lg:grid-cols-2">
                {/* Daily Usage Bar Chart */}
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                    <CardHeader>
                        <div className="flex items-center gap-2">
                            <BarChart3 className="h-4 w-4 text-muted-foreground" />
                            <CardTitle className="text-lg">Daily Activity</CardTitle>
                        </div>
                        <CardDescription>Requests over the last 7 days</CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="flex h-48 items-end gap-2 pt-4">
                            {data.daily.map((day) => {
                                const height = (day.requests / maxDailyRequests) * 100
                                const dateLabel = new Date(day.date).toLocaleDateString(undefined, { weekday: 'short' })
                                return (
                                    <div key={day.date} className="group relative flex flex-1 flex-col items-center gap-2">
                                        <div
                                            className="w-full rounded-t-sm bg-blue-500/40 transition-all hover:bg-blue-500/60"
                                            style={{ height: `${Math.max(height, 4)}%` }}
                                        >
                                            <div className="absolute -top-8 left-1/2 -translate-x-1/2 rounded bg-popover px-2 py-1 text-[10px] opacity-0 transition-opacity group-hover:opacity-100">
                                                {day.requests} reqs
                                            </div>
                                        </div>
                                        <span className="text-[10px] text-muted-foreground">{dateLabel}</span>
                                    </div>
                                )
                            })}
                        </div>
                    </CardContent>
                </Card>

                {/* Provider Breakdown */}
                <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                    <CardHeader>
                        <div className="flex items-center gap-2">
                            <PieChart className="h-4 w-4 text-muted-foreground" />
                            <CardTitle className="text-lg">Providers</CardTitle>
                        </div>
                        <CardDescription>Requests by backend provider</CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-4 pt-2">
                            {providerEntries.length === 0 && (
                                <div className="py-8 text-center text-sm text-muted-foreground">No provider data available</div>
                            )}
                            {providerEntries.map(([provider, count]) => {
                                const percentage = (count / totalProviderRequests) * 100
                                return (
                                    <div key={provider} className="space-y-1">
                                        <div className="flex items-center justify-between text-xs">
                                            <span className="font-medium capitalize">{provider}</span>
                                            <span className="text-muted-foreground">
                                                {count} ({percentage.toFixed(1)}%)
                                            </span>
                                        </div>
                                        <div className="h-2 w-full overflow-hidden rounded-full bg-muted/30">
                                            <div
                                                className="h-full bg-blue-500/50"
                                                style={{ width: `${percentage}%` }}
                                            />
                                        </div>
                                    </div>
                                )
                            })}
                        </div>
                    </CardContent>
                </Card>
            </div>

            {/* Model Breakdown Table */}
            <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
                <CardHeader>
                    <CardTitle className="text-lg">Model Breakdown</CardTitle>
                    <CardDescription>Usage statistics per model</CardDescription>
                </CardHeader>
                <CardContent>
                    <div className="overflow-hidden rounded-md border border-border/50">
                        <table className="w-full text-sm">
                            <thead className="bg-muted/40">
                                <tr>
                                    <th className="px-4 py-3 text-left font-medium">Model</th>
                                    <th className="px-4 py-3 text-right font-medium">Requests</th>
                                    <th className="px-4 py-3 text-right font-medium">Share</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-border/40">
                                {Object.entries(data.by_model)
                                    .sort((a, b) => b[1] - a[1])
                                    .map(([model, count]) => (
                                        <tr key={model} className="hover:bg-muted/20 transition-colors">
                                            <td className="px-4 py-3 font-mono text-xs">{model}</td>
                                            <td className="px-4 py-3 text-right">{count.toLocaleString()}</td>
                                            <td className="px-4 py-3 text-right text-muted-foreground">
                                                {((count / data.total_requests) * 100).toFixed(1)}%
                                            </td>
                                        </tr>
                                    ))}
                                {Object.keys(data.by_model).length === 0 && (
                                    <tr>
                                        <td colSpan={3} className="px-4 py-8 text-center text-muted-foreground">
                                            No model usage recorded yet.
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}
