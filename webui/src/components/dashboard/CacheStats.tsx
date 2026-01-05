import { useEffect, useState, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import {
  Database,
  HardDrive,
  Trash2,
  RefreshCw,
  Zap,
  TrendingUp,
  Hash,
  Loader2,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { cn } from '@/lib/utils'

interface ResponseCacheStats {
  hits: number
  misses: number
  evictions: number
  size: number
  total_saved: number
  enabled: boolean
}

interface PromptCacheStats {
  hits: number
  misses: number
  evictions: number
  size: number
  unique_prompts: number
  total_requests: number
  estimated_tokens_saved: number
  top_providers: Record<string, number>
  enabled: boolean
}

interface TopPrompt {
  hash: string
  token_estimate: number
  hit_count: number
  providers: Record<string, number>
  prompt_preview: string
}

const formatNumber = (n: number): string => {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

const hitRate = (hits: number, misses: number): string => {
  const total = hits + misses
  if (total === 0) return '0%'
  return `${((hits / total) * 100).toFixed(1)}%`
}

export function CacheStats() {
  const { mgmtKey, mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()

  const [responseCache, setResponseCache] = useState<ResponseCacheStats | null>(null)
  const [promptCache, setPromptCache] = useState<PromptCacheStats | null>(null)
  const [topPrompts, setTopPrompts] = useState<TopPrompt[]>([])
  const [loading, setLoading] = useState(true)
  const [topPromptsOpen, setTopPromptsOpen] = useState(false)

  const isRunning = status?.running ?? false

  const loadStats = useCallback(async () => {
    if (!isRunning) return
    try {
      setLoading(true)
      const [respStats, promptStats, topData] = await Promise.all([
        mgmtFetch('/v0/management/cache/stats'),
        mgmtFetch('/v0/management/prompt-cache/stats'),
        mgmtFetch('/v0/management/prompt-cache/top'),
      ])
      setResponseCache(respStats)
      setPromptCache(promptStats)
      setTopPrompts(topData.prompts || [])
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    } finally {
      setLoading(false)
    }
  }, [mgmtFetch, showToast, isRunning])

  const toggleResponseCacheEnabled = async (enabled: boolean) => {
    try {
      await mgmtFetch('/v0/management/cache/enabled', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      })
      setResponseCache((prev) => prev ? { ...prev, enabled } : null)
      showToast(`Response cache ${enabled ? 'enabled' : 'disabled'}`, 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const clearResponseCache = async () => {
    try {
      await mgmtFetch('/v0/management/cache/clear', { method: 'POST' })
      await loadStats()
      showToast('Response cache cleared', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const togglePromptCacheEnabled = async (enabled: boolean) => {
    try {
      await mgmtFetch('/v0/management/prompt-cache/enabled', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      })
      setPromptCache((prev) => prev ? { ...prev, enabled } : null)
      showToast(`Prompt cache ${enabled ? 'enabled' : 'disabled'}`, 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const clearPromptCache = async () => {
    try {
      await mgmtFetch('/v0/management/prompt-cache/clear', { method: 'POST' })
      await loadStats()
      showToast('Prompt cache cleared', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  useEffect(() => {
    if (mgmtKey && isRunning) {
      loadStats()
      const interval = setInterval(loadStats, 15000)
      return () => clearInterval(interval)
    } else if (!isRunning) {
      setResponseCache(null)
      setPromptCache(null)
      setTopPrompts([])
    }
  }, [mgmtKey, isRunning, loadStats])

  if (!mgmtKey) return null

  return (
    <div className="bg-gradient-to-b from-zinc-900 to-black border border-cyan-500/40 rounded-lg shadow-2xl shadow-cyan-500/10 overflow-hidden">
      {/* Header */}
      <div className="bg-gradient-to-r from-cyan-950/80 via-cyan-900/60 to-cyan-950/80 border-b border-cyan-500/40 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Database className={cn('h-5 w-5', isRunning ? 'text-cyan-400' : 'text-muted')} />
            <span className="text-sm font-mono font-bold text-cyan-300 uppercase tracking-widest">
              Cache Statistics
            </span>
            {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-cyan-500/60" />}
          </div>
          <Button
            size="sm"
            variant="ghost"
            onClick={loadStats}
            disabled={!isRunning}
            className="gap-1.5 text-xs font-mono bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 border border-cyan-500/30"
          >
            <RefreshCw className="h-3 w-3" />
            REFRESH
          </Button>
        </div>
      </div>

      {/* Content */}
      <div className="p-4 space-y-6">
        {!isRunning && (
          <div className="py-8 text-center text-muted-foreground border border-dashed border-cyan-500/20 rounded bg-black/20">
            <p className="text-sm uppercase tracking-widest font-mono">Engine Offline</p>
            <p className="text-xs mt-1">Start the proxy engine to view cache statistics</p>
          </div>
        )}

        {isRunning && loading && !responseCache && (
          <div className="py-8 flex justify-center">
            <Loader2 className="h-8 w-8 animate-spin text-cyan-500/60" />
          </div>
        )}

        {isRunning && responseCache && (
          <>
            {/* Response Cache Section */}
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <HardDrive className="h-4 w-4 text-cyan-400" />
                  <span className="text-xs font-mono text-cyan-300 uppercase tracking-wider">
                    Response Cache
                  </span>
                </div>
                <div className="flex items-center gap-4">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="response-cache-toggle"
                      checked={responseCache.enabled}
                      onCheckedChange={toggleResponseCacheEnabled}
                    />
                    <Label htmlFor="response-cache-toggle" className="text-xs font-mono text-cyan-300 cursor-pointer">
                      {responseCache.enabled ? 'ON' : 'OFF'}
                    </Label>
                  </div>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={clearResponseCache}
                    disabled={responseCache.size === 0}
                    className="gap-1 text-xs font-mono bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/30"
                  >
                    <Trash2 className="h-3 w-3" />
                    CLEAR
                  </Button>
                </div>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                <StatCard label="Entries" value={formatNumber(responseCache.size)} icon={<Database className="h-4 w-4" />} />
                <StatCard label="Hits" value={formatNumber(responseCache.hits)} icon={<Zap className="h-4 w-4" />} accent="green" />
                <StatCard label="Misses" value={formatNumber(responseCache.misses)} icon={<Hash className="h-4 w-4" />} />
                <StatCard label="Hit Rate" value={hitRate(responseCache.hits, responseCache.misses)} icon={<TrendingUp className="h-4 w-4" />} accent="green" />
                <StatCard label="Tokens Saved" value={formatNumber(responseCache.total_saved)} icon={<Zap className="h-4 w-4" />} accent="amber" />
              </div>
            </div>

            {/* Prompt Cache Section */}
            {promptCache && (
              <div className="space-y-4 pt-4 border-t border-cyan-500/20">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Hash className="h-4 w-4 text-purple-400" />
                    <span className="text-xs font-mono text-purple-300 uppercase tracking-wider">
                      Prompt Cache
                    </span>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="flex items-center gap-2">
                      <Switch
                        id="prompt-cache-toggle"
                        checked={promptCache.enabled}
                        onCheckedChange={togglePromptCacheEnabled}
                      />
                      <Label htmlFor="prompt-cache-toggle" className="text-xs font-mono text-purple-300 cursor-pointer">
                        {promptCache.enabled ? 'ON' : 'OFF'}
                      </Label>
                    </div>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={clearPromptCache}
                      disabled={promptCache.size === 0}
                      className="gap-1 text-xs font-mono bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/30"
                    >
                      <Trash2 className="h-3 w-3" />
                      CLEAR
                    </Button>
                  </div>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                  <StatCard label="Entries" value={formatNumber(promptCache.size)} icon={<Database className="h-4 w-4" />} color="purple" />
                  <StatCard label="Hits" value={formatNumber(promptCache.hits)} icon={<Zap className="h-4 w-4" />} color="purple" accent="green" />
                  <StatCard label="Misses" value={formatNumber(promptCache.misses)} icon={<Hash className="h-4 w-4" />} color="purple" />
                  <StatCard label="Hit Rate" value={hitRate(promptCache.hits, promptCache.misses)} icon={<TrendingUp className="h-4 w-4" />} color="purple" accent="green" />
                  <StatCard label="Tokens Saved" value={formatNumber(promptCache.estimated_tokens_saved)} icon={<Zap className="h-4 w-4" />} color="purple" accent="amber" />
                </div>

                {/* Provider breakdown */}
                {Object.keys(promptCache.top_providers || {}).length > 0 && (
                  <div className="bg-black/40 border border-purple-500/20 rounded p-3">
                    <span className="text-xs font-mono text-purple-500/60 uppercase">Hits by Provider</span>
                    <div className="flex flex-wrap gap-2 mt-2">
                      {Object.entries(promptCache.top_providers).map(([provider, count]) => (
                        <span
                          key={provider}
                          className="px-2 py-1 text-xs font-mono bg-purple-500/20 text-purple-300 rounded"
                        >
                          {provider}: {formatNumber(count)}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {/* Top Prompts Collapsible */}
                {topPrompts.length > 0 && (
                  <div className="border border-purple-500/30 rounded bg-black/40">
                    <button
                      onClick={() => setTopPromptsOpen(!topPromptsOpen)}
                      className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-purple-500/10 transition-colors"
                    >
                      {topPromptsOpen ? (
                        <ChevronDown className="h-4 w-4 text-purple-400" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-purple-400" />
                      )}
                      <span className="text-xs font-mono text-purple-300 uppercase tracking-wider">
                        Top Cached Prompts
                      </span>
                      <span className="ml-auto text-xs font-mono bg-purple-500/20 text-purple-300 px-2 py-0.5 rounded">
                        {topPrompts.length}
                      </span>
                    </button>
                    {topPromptsOpen && (
                      <div className="px-3 pb-3">
                        <div className="bg-black/60 border border-purple-500/20 rounded max-h-48 overflow-auto">
                          <div className="divide-y divide-purple-500/10">
                            {topPrompts.map((prompt) => (
                              <div key={prompt.hash} className="px-3 py-2 hover:bg-purple-500/5">
                                <div className="flex items-start justify-between gap-2">
                                  <span className="text-xs font-mono text-purple-200/80 break-words flex-1">
                                    {prompt.prompt_preview}
                                  </span>
                                  <div className="flex items-center gap-2 shrink-0">
                                    <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-green-500/20 text-green-300">
                                      {prompt.hit_count} hits
                                    </span>
                                    <span className="text-xs font-mono px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300">
                                      ~{formatNumber(prompt.token_estimate)} tok
                                    </span>
                                  </div>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  icon,
  color = 'cyan',
  accent,
}: {
  label: string
  value: string
  icon: React.ReactNode
  color?: 'cyan' | 'purple'
  accent?: 'green' | 'amber'
}) {
  const borderColor = color === 'purple' ? 'border-purple-500/30' : 'border-cyan-500/30'
  const labelColor = color === 'purple' ? 'text-purple-500/60' : 'text-cyan-500/60'
  const iconColor = color === 'purple' ? 'text-purple-400' : 'text-cyan-400'

  let valueColor = color === 'purple' ? 'text-purple-200' : 'text-cyan-200'
  if (accent === 'green') valueColor = 'text-green-400'
  if (accent === 'amber') valueColor = 'text-amber-400'

  return (
    <div className={cn('bg-black/40 border rounded p-3', borderColor)}>
      <div className="flex items-center gap-1.5 mb-1">
        <span className={iconColor}>{icon}</span>
        <span className={cn('text-xs font-mono uppercase', labelColor)}>{label}</span>
      </div>
      <div className={cn('text-lg font-mono font-bold', valueColor)}>{value}</div>
    </div>
  )
}
