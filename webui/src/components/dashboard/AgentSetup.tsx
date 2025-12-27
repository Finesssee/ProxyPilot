import { useEffect, useState, useCallback } from 'react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Terminal, Check, AlertTriangle, Copy, Loader2, Wrench, RefreshCw, Info, Undo2 } from 'lucide-react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'

interface Agent {
    id: string
    name: string
    detected: boolean
    configured: boolean
    configInstructions?: string
    envVars?: Record<string, string>
    canAutoConfigure?: boolean
}

export function AgentSetup() {
    const { mgmtFetch, showToast, status, isMgmtLoading, pp_detect_agents, pp_configure_agent, pp_unconfigure_agent, isDesktop } = useProxyContext()
    const [agents, setAgents] = useState<Agent[]>([])
    const [loading, setLoading] = useState(false)
    const [configuring, setConfiguring] = useState<string | null>(null)
    const [unconfiguring, setUnconfiguring] = useState<string | null>(null)
    const isRunning = status?.running ?? false

    const fetchAgents = useCallback(async () => {
        if (!isRunning) return
        setLoading(true)
        try {
            let data: Agent[] = []
            if (isDesktop && pp_detect_agents) {
                data = await pp_detect_agents()
            } else {
                const res = await mgmtFetch('/v0/management/agents')
                data = res.agents || []
            }
            setAgents(data)
        } catch (e) {
            if (!(e instanceof EngineOfflineError)) {
                console.error('Failed to fetch agents:', e)
            }
        }
        setLoading(false)
    }, [mgmtFetch, isRunning, isDesktop, pp_detect_agents])

    useEffect(() => {
        if (isRunning) {
            fetchAgents()
        } else {
            setAgents([])
        }
    }, [isRunning, fetchAgents])

    const copyEnvVars = (envVars: Record<string, string>) => {
        const text = Object.entries(envVars)
            .map(([key, value]) => `${key}=${value}`)
            .join('\n')
        navigator.clipboard.writeText(text)
        showToast('Environment variables copied to clipboard', 'success')
    }

    // Agents that use shell profile (need restart) vs config files (immediate)
    const needsShellRestart = (id: string) => ['claude-code', 'aider', 'gemini-cli'].includes(id)

    const handleConfigure = async (agent: Agent) => {
        if (!pp_configure_agent || !agent.id) return
        setConfiguring(agent.id)
        try {
            await pp_configure_agent(agent.id)
            const msg = needsShellRestart(agent.id)
                ? `${agent.name} configured! Restart shell to apply.`
                : `${agent.name} configured!`
            showToast(msg, 'success')
            await fetchAgents()
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        }
        setConfiguring(null)
    }

    const handleUnconfigure = async (agent: Agent) => {
        if (!pp_unconfigure_agent || !agent.id) return
        setUnconfiguring(agent.id)
        try {
            await pp_unconfigure_agent(agent.id)
            const msg = needsShellRestart(agent.id)
                ? `${agent.name} configuration removed. Restart shell to apply.`
                : `${agent.name} configuration removed.`
            showToast(msg, 'success')
            await fetchAgents()
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        }
        setUnconfiguring(null)
    }

    return (
        <div className="rounded-lg border border-[var(--border-default)] bg-[var(--bg-panel)] shadow-xl overflow-hidden">
            {/* Header */}
            <div className="flex items-center gap-2 px-4 py-3 bg-[var(--bg-elevated)] border-b border-[var(--border-default)]">
                <Terminal className={`h-4 w-4 text-[var(--accent-primary)] ${loading ? 'animate-pulse' : ''}`} />
                <span className="font-mono text-xs font-bold uppercase tracking-widest text-[var(--text-secondary)]">
                    CLI Agent Detection
                </span>
                {isMgmtLoading && <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />}
                <div className="flex-1" />
                <button
                    onClick={fetchAgents}
                    disabled={loading || !isRunning}
                    className="p-1.5 rounded hover:bg-[var(--bg-void)] text-[var(--text-muted)] hover:text-[var(--accent-primary)] transition-colors disabled:opacity-50"
                >
                    <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
                </button>
            </div>

            <div className="p-4">
                {!isRunning ? (
                    <div className="flex flex-col items-center justify-center py-8 text-center">
                        <div className="font-mono text-sm text-[var(--text-muted)] uppercase tracking-wider mb-1">
                            Engine Offline
                        </div>
                        <div className="text-xs text-[var(--text-muted)]">
                            Start the proxy engine to detect agents
                        </div>
                    </div>
                ) : agents.length === 0 && !loading ? (
                    <div className="text-center py-8 text-[var(--text-muted)] font-mono text-xs uppercase">
                        No agents detected
                    </div>
                ) : (
                    <div className="grid gap-3">
                        {agents.map((agent) => (
                            <div
                                key={agent.name}
                                className={`
                                    flex items-center gap-4 p-3 rounded border transition-all
                                    ${agent.detected
                                        ? 'bg-[var(--bg-elevated)]/50 border-[var(--border-default)]'
                                        : 'bg-[var(--bg-void)]/30 border-[var(--border-subtle)] opacity-60'
                                    }
                                `}
                            >
                                {/* Status Icon */}
                                <div className="shrink-0">
                                    {agent.configured ? (
                                        <div className="p-1.5 rounded-full bg-[var(--status-online)]/10 text-[var(--status-online)]">
                                            <Check className="h-4 w-4" />
                                        </div>
                                    ) : agent.detected ? (
                                        <div className="p-1.5 rounded-full bg-[var(--status-warning)]/10 text-[var(--status-warning)]">
                                            <AlertTriangle className="h-4 w-4" />
                                        </div>
                                    ) : (
                                        <div className="p-1.5 rounded-full bg-[var(--bg-void)] text-[var(--text-muted)]">
                                            <Terminal className="h-4 w-4" />
                                        </div>
                                    )}
                                </div>

                                {/* Info */}
                                <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2">
                                        <span className="font-mono text-sm font-bold text-[var(--text-primary)]">
                                            {agent.name}
                                        </span>
                                        {agent.detected && !agent.configured && (
                                            <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-[var(--status-warning)]/10 text-[var(--status-warning)] border border-[var(--status-warning)]/20">
                                                Needs Config
                                            </span>
                                        )}
                                    </div>
                                    <div className="text-[10px] font-mono text-[var(--text-muted)] truncate">
                                        {agent.detected
                                            ? (agent.configured ? 'Fully configured and ready' : 'Detected but not configured')
                                            : 'Not found on this system'
                                        }
                                    </div>
                                </div>

                                {/* Actions */}
                                <div className="flex items-center gap-2">
                                    {/* Auto-Configure Button */}
                                    {agent.detected && !agent.configured && agent.canAutoConfigure && isDesktop && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <button
                                                    onClick={() => handleConfigure(agent)}
                                                    disabled={configuring === agent.id}
                                                    className="flex items-center gap-1.5 px-2.5 py-1.5 rounded border border-[var(--accent-primary)] bg-[var(--accent-primary)]/10 text-[var(--accent-primary)] hover:bg-[var(--accent-primary)]/20 transition-all disabled:opacity-50 font-mono text-xs uppercase tracking-wider"
                                                >
                                                    {configuring === agent.id ? (
                                                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                                    ) : (
                                                        <Wrench className="h-3.5 w-3.5" />
                                                    )}
                                                    Configure
                                                </button>
                                            </TooltipTrigger>
                                            <TooltipContent>Auto-configure {agent.name} for ProxyPilot</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {/* Unconfigure Button */}
                                    {agent.detected && agent.configured && isDesktop && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <button
                                                    onClick={() => handleUnconfigure(agent)}
                                                    disabled={unconfiguring === agent.id}
                                                    className="flex items-center gap-1.5 px-2.5 py-1.5 rounded border border-[var(--status-error)] bg-[var(--status-error)]/10 text-[var(--status-error)] hover:bg-[var(--status-error)]/20 transition-all disabled:opacity-50 font-mono text-xs uppercase tracking-wider"
                                                >
                                                    {unconfiguring === agent.id ? (
                                                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                                    ) : (
                                                        <Undo2 className="h-3.5 w-3.5" />
                                                    )}
                                                    Reset
                                                </button>
                                            </TooltipTrigger>
                                            <TooltipContent>Remove ProxyPilot configuration from {agent.name}</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {agent.envVars && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <button
                                                    onClick={() => copyEnvVars(agent.envVars!)}
                                                    className="p-2 rounded border border-[var(--border-default)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)] transition-all"
                                                >
                                                    <Copy className="h-3.5 w-3.5" />
                                                </button>
                                            </TooltipTrigger>
                                            <TooltipContent>Copy Environment Variables</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {agent.configInstructions && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <button
                                                    onClick={() => {
                                                        showToast(agent.configInstructions!, 'success')
                                                    }}
                                                    className="p-2 rounded border border-[var(--border-default)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:border-[var(--accent-primary)] hover:text-[var(--accent-primary)] transition-all"
                                                >
                                                    <Info className="h-3.5 w-3.5" />
                                                </button>
                                            </TooltipTrigger>
                                            <TooltipContent>View Setup Instructions</TooltipContent>
                                        </Tooltip>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
