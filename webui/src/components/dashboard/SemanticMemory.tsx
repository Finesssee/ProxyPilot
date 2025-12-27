import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { Loader2 } from 'lucide-react'

interface SemanticHealthData {
  status?: string
  model?: string
  version?: string
  model_present?: boolean
  latency_ms?: number
  queue?: {
    queued: number
    dropped: number
    processed: number
    failed: number
  }
}

interface SemanticNamespace {
  key: string
  label?: string
}

interface SemanticItem {
  ts?: string
  source?: string
  role?: string
  text?: string
}

export function SemanticMemory() {
  const { mgmtKey, mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()

  const [semanticHealth, setSemanticHealth] = useState<SemanticHealthData | null>(null)
  const [semanticNamespaces, setSemanticNamespaces] = useState<SemanticNamespace[]>([])
  const [semanticNamespace, setSemanticNamespace] = useState('')
  const [semanticLimit, setSemanticLimit] = useState(50)
  const [semanticItems, setSemanticItems] = useState('')

  const isRunning = status?.running ?? false

  const loadSemanticHealth = useCallback(async () => {
    try {
      const res = await mgmtFetch('/v0/management/semantic/health')
      setSemanticHealth(res)
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }, [mgmtFetch, showToast])

  const loadSemanticNamespaces = useCallback(async () => {
    try {
      const res = await mgmtFetch('/v0/management/semantic/namespaces')
      const namespaces = res.namespaces || []
      setSemanticNamespaces(namespaces)
      if (!semanticNamespace && namespaces.length > 0) {
        setSemanticNamespace(namespaces[0].key)
      }
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }, [mgmtFetch, showToast, semanticNamespace])

  const loadSemanticItems = useCallback(async () => {
    if (!semanticNamespace) {
      setSemanticItems('Select a namespace.')
      return
    }
    try {
      const res = await mgmtFetch(
        `/v0/management/semantic/items?namespace=${encodeURIComponent(semanticNamespace)}&limit=${encodeURIComponent(semanticLimit)}`
      )
      const items: SemanticItem[] = res.items || []
      if (!Array.isArray(items) || items.length === 0) {
        setSemanticItems('No items.')
        return
      }
      const lines = items.map((it) => {
        const ts = it.ts || ''
        const src = it.source || ''
        const role = it.role || ''
        const text = (it.text || '').toString()
        return `[${ts}][${src}][${role}] ${text}`
      })
      setSemanticItems(lines.join('\n\n'))
    } catch (e) {
      if (!(e instanceof EngineOfflineError)) {
        showToast(e instanceof Error ? e.message : String(e), 'error')
      }
    }
  }, [mgmtFetch, showToast, semanticNamespace, semanticLimit])

  // Load initial data when mgmtKey is available
  useEffect(() => {
    if (mgmtKey && isRunning) {
      loadSemanticHealth()
      loadSemanticNamespaces()
    } else if (!isRunning) {
      setSemanticHealth(null)
      setSemanticNamespaces([])
      setSemanticItems('')
    }
  }, [mgmtKey, isRunning, loadSemanticHealth, loadSemanticNamespaces])

  if (!mgmtKey) {
    return null
  }

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
      <CardHeader className="pb-4">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-lg">Semantic Memory</CardTitle>
            <CardDescription>Ollama embeddings + per-repo namespaces</CardDescription>
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
            {/* Health Status */}
            <div className="flex items-center gap-2">
              <Button size="sm" variant="outline" onClick={loadSemanticHealth}>
                Refresh
              </Button>
              <Badge variant={semanticHealth?.status === 'ok' ? 'default' : 'secondary'}>
                {semanticHealth?.status || 'unknown'}
              </Badge>
              <span className="text-xs text-muted-foreground">
                {semanticHealth?.model} {semanticHealth?.version ? `v${semanticHealth.version}` : ''}
              </span>
              <span className="text-xs text-muted-foreground">
                {semanticHealth?.model_present === false ? 'model missing' : ''}
              </span>
            </div>

            {/* Queue Statistics */}
            <div className="text-xs text-muted-foreground">
              {semanticHealth?.latency_ms ? `latency ${semanticHealth.latency_ms}ms` : 'latency n/a'}
              {semanticHealth?.queue
                ? ` · queue q${semanticHealth.queue.queued} d${semanticHealth.queue.dropped} p${semanticHealth.queue.processed} f${semanticHealth.queue.failed}`
                : ''}
            </div>

            {/* Namespace Selection and Items */}
            <div className="flex items-center gap-2">
              <select
                className="min-w-[200px] rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                value={semanticNamespace}
                onChange={(e) => setSemanticNamespace(e.target.value)}
              >
                {semanticNamespaces.length === 0 && <option value="">(no namespaces)</option>}
                {semanticNamespaces.map((n) => (
                  <option key={n.key} value={n.key}>
                    {n.label || n.key}
                  </option>
                ))}
              </select>
              <input
                type="number"
                min={10}
                max={200}
                className="w-20 rounded-md border border-border bg-background/60 px-2 py-1 text-xs"
                value={semanticLimit}
                onChange={(e) => setSemanticLimit(parseInt(e.target.value || '50', 10))}
              />
              <Button size="sm" variant="outline" onClick={loadSemanticItems}>
                Load
              </Button>
            </div>

            {/* Items Display */}
            <pre className="max-h-56 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
              {semanticItems || 'No items loaded.'}
            </pre>
          </>
        )}
      </CardContent>
    </Card>
  )
}
