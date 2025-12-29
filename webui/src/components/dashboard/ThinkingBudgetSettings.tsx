import { useState, useEffect, useCallback } from 'react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { Card, CardHeader, CardContent, CardDescription, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
    Brain,
    RefreshCw,
    Save,
    Zap,
    Gauge,
    Sparkles,
    Settings2,
    Check
} from 'lucide-react'

interface ThinkingBudgetData {
    mode: 'low' | 'medium' | 'high' | 'custom'
    custom_tokens: number
    enabled: boolean
    effective_tokens: number
    presets: Record<string, number>
}

const PRESET_INFO: Record<string, { icon: React.ReactNode; label: string; description: string; color: string }> = {
    low: {
        icon: <Zap className="h-4 w-4" />,
        label: 'Low',
        description: '2,048 tokens - Fast, minimal reasoning',
        color: 'text-amber-400'
    },
    medium: {
        icon: <Gauge className="h-4 w-4" />,
        label: 'Medium',
        description: '8,192 tokens - Balanced reasoning',
        color: 'text-blue-400'
    },
    high: {
        icon: <Sparkles className="h-4 w-4" />,
        label: 'High',
        description: '32,768 tokens - Deep reasoning',
        color: 'text-purple-400'
    },
    custom: {
        icon: <Settings2 className="h-4 w-4" />,
        label: 'Custom',
        description: 'Set your own token budget',
        color: 'text-emerald-400'
    }
}

export function ThinkingBudgetSettings() {
    const { mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()
    const [data, setData] = useState<ThinkingBudgetData | null>(null)
    const [selectedMode, setSelectedMode] = useState<string>('medium')
    const [customTokens, setCustomTokens] = useState<number>(16000)
    const [enabled, setEnabled] = useState(true)
    const [isDirty, setIsDirty] = useState(false)
    const [saving, setSaving] = useState(false)
    const isRunning = status?.running ?? false

    const fetchData = useCallback(async () => {
        try {
            const res = await mgmtFetch('/v0/management/thinking-budget')
            setData(res)
            setSelectedMode(res.mode || 'medium')
            setCustomTokens(res.custom_tokens || 16000)
            setEnabled(res.enabled !== false)
            setIsDirty(false)
        } catch (e) {
            if (!(e instanceof EngineOfflineError)) {
                console.error('Failed to fetch thinking budget:', e)
            }
        }
    }, [mgmtFetch])

    useEffect(() => {
        if (isRunning) {
            fetchData()
        }
    }, [isRunning, fetchData])

    const handleModeChange = (mode: string) => {
        setSelectedMode(mode)
        setIsDirty(true)
    }

    const handleCustomTokensChange = (value: string) => {
        const num = parseInt(value, 10)
        if (!isNaN(num) && num > 0) {
            setCustomTokens(num)
            setIsDirty(true)
        }
    }

    const handleEnabledChange = () => {
        setEnabled(!enabled)
        setIsDirty(true)
    }

    const handleSave = async () => {
        setSaving(true)
        try {
            await mgmtFetch('/v0/management/thinking-budget', {
                method: 'PUT',
                body: JSON.stringify({
                    mode: selectedMode,
                    custom_tokens: customTokens,
                    enabled: enabled
                })
            })
            showToast('Thinking budget settings saved', 'success')
            setIsDirty(false)
            await fetchData()
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        }
        setSaving(false)
    }

    const getEffectiveTokens = () => {
        if (selectedMode === 'custom') {
            return customTokens
        }
        return data?.presets?.[selectedMode] || 8192
    }

    if (!isRunning) {
        return (
            <Card className="backdrop-blur-sm bg-[var(--bg-void)] border-[var(--border-default)] shadow-xl">
                <CardHeader className="pb-3 border-b border-[var(--border-subtle)] bg-[var(--bg-panel)]">
                    <div className="flex items-center gap-2">
                        <Brain className="h-4 w-4 text-[var(--text-muted)]" />
                        <CardTitle className="text-sm font-mono uppercase tracking-wider">Thinking Budget</CardTitle>
                    </div>
                </CardHeader>
                <CardContent className="p-8 text-center text-[var(--text-muted)]">
                    <p className="text-xs uppercase tracking-widest">Engine Offline</p>
                </CardContent>
            </Card>
        )
    }

    return (
        <Card className="backdrop-blur-sm bg-[var(--bg-void)] border-[var(--border-default)] shadow-xl">
            <CardHeader className="pb-3 border-b border-[var(--border-subtle)] bg-[var(--bg-panel)]">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <Brain className={`h-4 w-4 ${enabled ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)]'}`} />
                        <CardTitle className="text-sm font-mono uppercase tracking-wider">Thinking Budget</CardTitle>
                        {isMgmtLoading && <RefreshCw className="h-3 w-3 animate-spin text-muted-foreground" />}
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={fetchData}
                            disabled={isMgmtLoading}
                            className="text-xs h-7 font-mono gap-1"
                        >
                            <RefreshCw className="h-3 w-3" />
                            REFRESH
                        </Button>
                        <Button
                            size="sm"
                            onClick={handleSave}
                            disabled={!isDirty || saving}
                            className={`text-xs h-7 font-mono gap-1 ${isDirty ? 'bg-[var(--accent-primary)] text-white' : ''}`}
                        >
                            {saving ? <RefreshCw className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
                            SAVE
                        </Button>
                    </div>
                </div>
                <CardDescription className="text-xs text-[var(--text-muted)]">
                    Configure default thinking budget for reasoning models (Claude, Gemini with thinking)
                </CardDescription>
            </CardHeader>

            <CardContent className="p-4 space-y-4">
                {/* Enable/Disable Toggle */}
                <div className="flex items-center justify-between p-3 rounded-lg bg-[var(--bg-panel)] border border-[var(--border-subtle)]">
                    <div>
                        <div className="font-mono text-sm text-[var(--text-primary)]">Enable Thinking</div>
                        <div className="text-xs text-[var(--text-muted)]">Allow models to use extended reasoning</div>
                    </div>
                    <button
                        onClick={handleEnabledChange}
                        className={`relative w-12 h-6 rounded-full transition-colors ${enabled ? 'bg-[var(--accent-primary)]' : 'bg-[var(--bg-elevated)]'
                            }`}
                    >
                        <span
                            className={`absolute top-1 w-4 h-4 rounded-full bg-white transition-transform ${enabled ? 'left-7' : 'left-1'
                                }`}
                        />
                    </button>
                </div>

                {/* Preset Selection */}
                <div className="space-y-2">
                    <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase tracking-wider">
                        Budget Preset
                    </label>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                        {Object.entries(PRESET_INFO).map(([mode, info]) => (
                            <button
                                key={mode}
                                onClick={() => handleModeChange(mode)}
                                disabled={!enabled}
                                className={`relative p-3 rounded-lg border transition-all text-left ${selectedMode === mode
                                        ? `border-[var(--accent-primary)] bg-[var(--accent-primary)]/10`
                                        : 'border-[var(--border-subtle)] bg-[var(--bg-panel)] hover:border-[var(--border-default)]'
                                    } ${!enabled ? 'opacity-50 cursor-not-allowed' : ''}`}
                            >
                                {selectedMode === mode && (
                                    <div className="absolute top-2 right-2">
                                        <Check className="h-3 w-3 text-[var(--accent-primary)]" />
                                    </div>
                                )}
                                <div className={`flex items-center gap-2 ${info.color}`}>
                                    {info.icon}
                                    <span className="font-mono text-sm font-bold">{info.label}</span>
                                </div>
                                <div className="text-[10px] text-[var(--text-muted)] mt-1">
                                    {info.description}
                                </div>
                            </button>
                        ))}
                    </div>
                </div>

                {/* Custom Tokens Input */}
                {selectedMode === 'custom' && (
                    <div className="space-y-2">
                        <label className="block text-[10px] font-mono text-[var(--text-muted)] uppercase tracking-wider">
                            Custom Token Budget
                        </label>
                        <div className="flex items-center gap-2">
                            <input
                                type="number"
                                value={customTokens}
                                onChange={(e) => handleCustomTokensChange(e.target.value)}
                                disabled={!enabled}
                                min={1024}
                                max={128000}
                                step={1024}
                                className="flex-1 px-3 py-2 rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] font-mono text-sm disabled:opacity-50"
                            />
                            <span className="text-xs text-[var(--text-muted)]">tokens</span>
                        </div>
                        <div className="text-[10px] text-[var(--text-muted)]">
                            Recommended range: 1,024 - 128,000 tokens
                        </div>
                    </div>
                )}

                {/* Effective Budget Display */}
                <div className="p-3 rounded-lg bg-[var(--bg-elevated)] border border-[var(--border-subtle)]">
                    <div className="flex items-center justify-between">
                        <div className="text-xs text-[var(--text-muted)]">Effective Thinking Budget</div>
                        <div className={`font-mono text-lg font-bold ${enabled ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)]'}`}>
                            {enabled ? getEffectiveTokens().toLocaleString() : '0'} <span className="text-xs font-normal">tokens</span>
                        </div>
                    </div>
                </div>

                {/* Info */}
                <div className="text-xs text-[var(--text-muted)] p-3 rounded-lg bg-[var(--bg-panel)] border border-[var(--border-subtle)]">
                    <strong>Note:</strong> Thinking budget applies to models that support extended reasoning,
                    including Claude with thinking mode and Gemini Pro/Flash with thinking enabled.
                    Higher budgets allow for more thorough reasoning but consume more tokens.
                </div>
            </CardContent>
        </Card>
    )
}
