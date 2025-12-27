import { useState, useEffect } from 'react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Plus, Trash2, Pencil, Check, X, Search, ChevronDown, Loader2 } from 'lucide-react'

interface ModelMapping {
  from: string
  to: string
  provider: string
}

const PROVIDERS = ['anthropic', 'google', 'openai', 'azure', 'bedrock', 'vertex', 'other'] as const

function getProviderColor(provider: string): string {
  const p = provider.toLowerCase()
  if (p.includes('anthropic')) return 'oklch(0.60 0.15 35)'
  if (p.includes('google') || p.includes('vertex')) return 'oklch(0.55 0.18 250)'
  if (p.includes('openai') || p.includes('azure')) return 'oklch(0.55 0.12 145)'
  if (p.includes('bedrock') || p.includes('aws')) return 'oklch(0.58 0.14 50)'
  return 'oklch(0.50 0.08 280)'
}

function getProviderBgColor(provider: string): string {
  const p = provider.toLowerCase()
  if (p.includes('anthropic')) return 'oklch(0.60 0.15 35 / 0.15)'
  if (p.includes('google') || p.includes('vertex')) return 'oklch(0.55 0.18 250 / 0.15)'
  if (p.includes('openai') || p.includes('azure')) return 'oklch(0.55 0.12 145 / 0.15)'
  if (p.includes('bedrock') || p.includes('aws')) return 'oklch(0.58 0.14 50 / 0.15)'
  return 'oklch(0.50 0.08 280 / 0.15)'
}

export function ModelMappings() {
  const { mgmtKey, mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()

  const [modelMappings, setModelMappings] = useState<ModelMapping[]>([])
  const [showAddForm, setShowAddForm] = useState(false)
  const [newMapping, setNewMapping] = useState<ModelMapping>({ from: '', to: '', provider: '' })
  const [editingMapping, setEditingMapping] = useState<number | null>(null)
  const [editingMappingValues, setEditingMappingValues] = useState<ModelMapping>({ from: '', to: '', provider: '' })
  const [mappingTestInput, setMappingTestInput] = useState('')
  const [mappingTestResult, setMappingTestResult] = useState<{ model?: string; provider?: string; matched?: boolean } | null>(null)
  const [deletingIndex, setDeletingIndex] = useState<number | null>(null)
  const [newlyAddedIndex, setNewlyAddedIndex] = useState<number | null>(null)

  const isRunning = status?.running ?? false

  const fetchModelMappings = async () => {
    try {
      const data = await mgmtFetch('/v0/management/model-mappings')
      setModelMappings(data.mappings || [])
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        console.error('Failed to fetch model mappings:', e)
      }
    }
  }

  const addModelMapping = async () => {
    if (!newMapping.from.trim() || !newMapping.to.trim()) {
      showToast('Both "from" and "to" fields are required', 'error')
      return
    }
    try {
      await mgmtFetch('/v0/management/model-mappings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newMapping),
      })
      setNewMapping({ from: '', to: '', provider: '' })
      setShowAddForm(false)
      await fetchModelMappings()
      setNewlyAddedIndex(modelMappings.length)
      setTimeout(() => setNewlyAddedIndex(null), 500)
      showToast('Route added to flight plan', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const updateModelMapping = async (index: number, mapping: ModelMapping) => {
    try {
      await mgmtFetch(`/v0/management/model-mappings/${index}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(mapping),
      })
      setEditingMapping(null)
      await fetchModelMappings()
      showToast('Route updated', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const deleteModelMapping = async (index: number) => {
    setDeletingIndex(index)
    try {
      await mgmtFetch(`/v0/management/model-mappings/${index}`, {
        method: 'DELETE',
      })
      setTimeout(async () => {
        await fetchModelMappings()
        setDeletingIndex(null)
        showToast('Route removed', 'success')
      }, 300)
    } catch (e) {
      setDeletingIndex(null)
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const testModelMapping = async () => {
    if (!mappingTestInput.trim()) {
      setMappingTestResult({ matched: false })
      return
    }
    try {
      const data = await mgmtFetch(`/v0/management/model-mappings/test?model=${encodeURIComponent(mappingTestInput.trim())}`)
      setMappingTestResult({
        model: data.model || data.to || mappingTestInput,
        provider: data.provider,
        matched: data.model !== mappingTestInput || !!data.provider
      })
    } catch (e) {
      setMappingTestResult({ matched: false })
    }
  }

  useEffect(() => {
    if (mgmtKey && isRunning) {
      fetchModelMappings()
    } else if (!isRunning) {
      setModelMappings([])
    }
  }, [mgmtKey, isRunning])

  if (!mgmtKey) {
    return null
  }

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl overflow-hidden">
      {/* Header */}
      <div className="px-6 py-4 border-b border-border/50 bg-gradient-to-r from-orange-500/5 to-transparent">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="relative">
              <div className={`h-3 w-3 rounded-full ${isRunning ? 'bg-orange-500 animate-pulse' : 'bg-muted'}`} />
              {isRunning && <div className="absolute inset-0 h-3 w-3 rounded-full bg-orange-500/50 animate-ping" />}
            </div>
            <h2 className="text-base font-semibold tracking-wide uppercase text-foreground/90">
              Flight Plan
            </h2>
            {isMgmtLoading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
          </div>
          <Button
            size="sm"
            variant="outline"
            disabled={!isRunning}
            onClick={() => setShowAddForm(!showAddForm)}
            className="gap-1.5 text-xs border-orange-500/30 hover:border-orange-500/50 hover:bg-orange-500/10"
          >
            <Plus className="h-3.5 w-3.5" />
            Add Route
          </Button>
        </div>
      </div>

      <CardContent className="p-4 space-y-3">
        {!isRunning && (
          <div className="py-8 text-center text-muted-foreground">
            <p className="text-sm uppercase tracking-widest font-mono">⚠️ Engine Offline</p>
            <p className="text-xs mt-1">Start the proxy engine to manage model mappings</p>
          </div>
        )}

        {isRunning && (
          <>
            {/* Add Route Form - Slide in */}
            <div
              className={`overflow-hidden transition-all duration-300 ease-out ${showAddForm ? 'max-h-48 opacity-100' : 'max-h-0 opacity-0'
                }`}
            >
              <div className="p-4 rounded-lg bg-orange-500/5 border border-orange-500/20 space-y-3 mb-3">
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                      From Model
                    </label>
                    <input
                      type="text"
                      className="w-full rounded-md border border-border bg-background/80 px-3 py-2 text-sm font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-orange-500/30 focus:border-orange-500/50 transition-all"
                      placeholder="gpt-4"
                      value={newMapping.from}
                      onChange={(e) => setNewMapping({ ...newMapping, from: e.target.value })}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                      To Model
                    </label>
                    <input
                      type="text"
                      className="w-full rounded-md border border-border bg-background/80 px-3 py-2 text-sm font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-orange-500/30 focus:border-orange-500/50 transition-all"
                      placeholder="claude-3-opus"
                      value={newMapping.to}
                      onChange={(e) => setNewMapping({ ...newMapping, to: e.target.value })}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                      Provider
                    </label>
                    <div className="relative">
                      <select
                        className="w-full rounded-md border border-border bg-background/80 px-3 py-2 text-sm font-mono appearance-none cursor-pointer focus:outline-none focus:ring-2 focus:ring-orange-500/30 focus:border-orange-500/50 transition-all"
                        value={newMapping.provider}
                        onChange={(e) => setNewMapping({ ...newMapping, provider: e.target.value })}
                      >
                        <option value="">Select provider...</option>
                        {PROVIDERS.map((p) => (
                          <option key={p} value={p}>
                            {p.charAt(0).toUpperCase() + p.slice(1)}
                          </option>
                        ))}
                      </select>
                      <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
                    </div>
                  </div>
                </div>
                <div className="flex justify-end gap-2">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      setShowAddForm(false)
                      setNewMapping({ from: '', to: '', provider: '' })
                    }}
                    className="text-xs"
                  >
                    Cancel
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => addModelMapping().catch((e) => showToast(String(e), 'error'))}
                    className="text-xs bg-orange-500 hover:bg-orange-600 text-white"
                  >
                    Save Route
                  </Button>
                </div>
              </div>
            </div>

            {/* Routes List */}
            <div className="space-y-2">
              {modelMappings.length === 0 && !showAddForm && !isMgmtLoading && (
                <div className="py-8 text-center text-muted-foreground">
                  <div className="text-3xl mb-2">---</div>
                  <p className="text-sm">No routes configured</p>
                  <p className="text-xs mt-1">Add a route to start mapping models</p>
                </div>
              )}

              {isMgmtLoading && modelMappings.length === 0 && (
                <div className="py-8 text-center text-muted-foreground">
                  <Loader2 className="h-8 w-8 animate-spin mx-auto mb-2 opacity-20" />
                  <p className="text-xs uppercase tracking-widest font-mono opacity-50">Loading Flight Plan...</p>
                </div>
              )}

              {modelMappings.map((mapping, index) => (
                <div
                  key={index}
                  className={`group relative rounded-lg border transition-all duration-300 ${deletingIndex === index
                      ? 'opacity-0 scale-95 max-h-0 overflow-hidden border-transparent'
                      : newlyAddedIndex === index
                        ? 'opacity-0 animate-fade-in border-border/50 bg-card/80 hover:bg-card hover:border-border'
                        : 'border-border/50 bg-card/80 hover:bg-card hover:border-border'
                    }`}
                  style={{
                    animation: newlyAddedIndex === index ? 'fadeIn 0.3s ease-out forwards' : undefined
                  }}
                >
                  {editingMapping === index ? (
                    /* Edit Mode */
                    <div className="p-4 space-y-3">
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                        <div className="space-y-1">
                          <label className="text-xs text-muted-foreground">From</label>
                          <input
                            type="text"
                            className="w-full rounded-md border border-border bg-background/80 px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500/30 transition-all"
                            value={editingMappingValues.from}
                            onChange={(e) => setEditingMappingValues(v => ({ ...v, from: e.target.value }))}
                          />
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs text-muted-foreground">To</label>
                          <input
                            type="text"
                            className="w-full rounded-md border border-border bg-background/80 px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500/30 transition-all"
                            value={editingMappingValues.to}
                            onChange={(e) => setEditingMappingValues(v => ({ ...v, to: e.target.value }))}
                          />
                        </div>
                        <div className="space-y-1">
                          <label className="text-xs text-muted-foreground">Provider</label>
                          <div className="relative">
                            <select
                              className="w-full rounded-md border border-border bg-background/80 px-3 py-2 text-sm font-mono appearance-none cursor-pointer focus:outline-none focus:ring-2 focus:ring-orange-500/30 transition-all"
                              value={editingMappingValues.provider}
                              onChange={(e) => setEditingMappingValues(v => ({ ...v, provider: e.target.value }))}
                            >
                              <option value="">None</option>
                              {PROVIDERS.map((p) => (
                                <option key={p} value={p}>
                                  {p.charAt(0).toUpperCase() + p.slice(1)}
                                </option>
                              ))}
                            </select>
                            <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
                          </div>
                        </div>
                      </div>
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => setEditingMapping(null)}
                          className="h-8 px-3 text-xs"
                        >
                          <X className="h-3.5 w-3.5 mr-1" />
                          Cancel
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => updateModelMapping(index, editingMappingValues).catch((e) => showToast(String(e), 'error'))}
                          className="h-8 px-3 text-xs bg-green-600 hover:bg-green-700 text-white"
                        >
                          <Check className="h-3.5 w-3.5 mr-1" />
                          Save
                        </Button>
                      </div>
                    </div>
                  ) : (
                    /* View Mode */
                    <div className="flex items-center gap-3 px-4 py-3">
                      {/* From Model */}
                      <div className="flex-shrink-0 min-w-[100px]">
                        <code className="text-sm font-mono font-medium text-foreground/90">
                          {mapping.from}
                        </code>
                      </div>

                      {/* Animated Arrow */}
                      <div className="flex-shrink-0 flex items-center gap-0.5 text-orange-500/70 group-hover:text-orange-500 transition-colors">
                        <span className="inline-block w-8 h-[2px] bg-current rounded-full relative overflow-hidden">
                          <span className="absolute inset-0 bg-gradient-to-r from-transparent via-orange-300 to-transparent opacity-0 group-hover:opacity-100 group-hover:animate-flow" />
                        </span>
                        <span className="inline-block w-8 h-[2px] bg-current rounded-full relative overflow-hidden">
                          <span className="absolute inset-0 bg-gradient-to-r from-transparent via-orange-300 to-transparent opacity-0 group-hover:opacity-100 group-hover:animate-flow animation-delay-100" />
                        </span>
                        <svg
                          className="h-4 w-4 -ml-0.5 transition-transform group-hover:translate-x-0.5"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2.5"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        >
                          <path d="M5 12h14M12 5l7 7-7 7" />
                        </svg>
                      </div>

                      {/* To Model */}
                      <div className="flex-1 min-w-[100px]">
                        <code className="text-sm font-mono font-medium text-orange-400">
                          {mapping.to}
                        </code>
                      </div>

                      {/* Provider Badge */}
                      {mapping.provider && (
                        <div
                          className="flex-shrink-0 px-2.5 py-1 rounded text-xs font-semibold uppercase tracking-wider"
                          style={{
                            color: getProviderColor(mapping.provider),
                            backgroundColor: getProviderBgColor(mapping.provider),
                          }}
                        >
                          {mapping.provider}
                        </div>
                      )}

                      {/* Action Buttons */}
                      <div className="flex-shrink-0 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 w-8 p-0 hover:bg-muted"
                          onClick={() => {
                            setEditingMapping(index)
                            setEditingMappingValues({
                              from: mapping.from || '',
                              to: mapping.to || '',
                              provider: mapping.provider || '',
                            })
                          }}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 w-8 p-0 hover:bg-destructive/10 text-destructive"
                          onClick={() => deleteModelMapping(index).catch((e) => showToast(String(e), 'error'))}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>

            {/* Test Route - Destination Lookup */}
            <div className="mt-4 pt-4 border-t border-border/50">
              <div className="rounded-lg bg-muted/30 border border-border/50 p-4">
                <div className="flex items-center gap-3 flex-wrap">
                  <div className="flex items-center gap-2 flex-1 min-w-[200px]">
                    <Search className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                    <span className="text-xs text-muted-foreground whitespace-nowrap">Test Route:</span>
                    <input
                      type="text"
                      className="flex-1 min-w-[120px] rounded-md border border-border bg-background/80 px-3 py-1.5 text-sm font-mono placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-orange-500/30 transition-all"
                      placeholder="Enter model name..."
                      value={mappingTestInput}
                      onChange={(e) => {
                        setMappingTestInput(e.target.value)
                        if (!e.target.value.trim()) {
                          setMappingTestResult(null)
                        }
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          testModelMapping().catch((e) => showToast(String(e), 'error'))
                        }
                      }}
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => testModelMapping().catch((e) => showToast(String(e), 'error'))}
                      className="text-xs"
                    >
                      Lookup
                    </Button>
                  </div>

                  {/* Result Display */}
                  {mappingTestResult && (
                    <div className="flex items-center gap-2">
                      <span className="text-orange-500">---</span>
                      {mappingTestResult.matched ? (
                        <>
                          <code className="text-sm font-mono font-medium text-orange-400">
                            {mappingTestResult.model}
                          </code>
                          {mappingTestResult.provider && (
                            <span
                              className="px-2 py-0.5 rounded text-xs font-semibold uppercase"
                              style={{
                                color: getProviderColor(mappingTestResult.provider),
                                backgroundColor: getProviderBgColor(mappingTestResult.provider),
                              }}
                            >
                              {mappingTestResult.provider}
                            </span>
                          )}
                        </>
                      ) : (
                        <span className="text-sm text-muted-foreground italic">No match found</span>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </>
        )}
      </CardContent>

      {/* Custom animations */}
      <style>{`
        @keyframes fadeIn {
          from {
            opacity: 0;
            transform: translateY(-10px);
          }
          to {
            opacity: 1;
            transform: translateY(0);
          }
        }

        @keyframes flow {
          0% {
            transform: translateX(-100%);
          }
          100% {
            transform: translateX(100%);
          }
        }

        .animate-fade-in {
          animation: fadeIn 0.3s ease-out forwards;
        }

        .group:hover .group-hover\\:animate-flow {
          animation: flow 0.8s ease-in-out infinite;
        }

        .animation-delay-100 {
          animation-delay: 0.15s;
        }
      `}</style>
    </Card>
  )
}
