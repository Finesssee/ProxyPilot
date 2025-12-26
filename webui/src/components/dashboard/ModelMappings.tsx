import { useState, useEffect } from 'react'
import { useProxyContext } from '@/hooks/useProxyContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ArrowRight, Plus, Trash2, Pencil, Check, X } from 'lucide-react'

interface ModelMapping {
  from: string
  to: string
  provider: string
}

export function ModelMappings() {
  const { mgmtKey, mgmtFetch, showToast } = useProxyContext()

  const [modelMappings, setModelMappings] = useState<ModelMapping[]>([])
  const [newMapping, setNewMapping] = useState<ModelMapping>({ from: '', to: '', provider: '' })
  const [editingMapping, setEditingMapping] = useState<number | null>(null)
  const [editingMappingValues, setEditingMappingValues] = useState<ModelMapping>({ from: '', to: '', provider: '' })
  const [mappingTestInput, setMappingTestInput] = useState('')
  const [mappingTestResult, setMappingTestResult] = useState('')

  const fetchModelMappings = async () => {
    try {
      const data = await mgmtFetch('/v0/management/model-mappings')
      setModelMappings(data.mappings || [])
    } catch (e) {
      console.error('Failed to fetch model mappings:', e)
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
      await fetchModelMappings()
      showToast('Mapping added', 'success')
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
      showToast('Mapping updated', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const deleteModelMapping = async (index: number) => {
    try {
      await mgmtFetch(`/v0/management/model-mappings/${index}`, {
        method: 'DELETE',
      })
      await fetchModelMappings()
      showToast('Mapping deleted', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const testModelMapping = async () => {
    if (!mappingTestInput.trim()) {
      setMappingTestResult('Enter a model name to test')
      return
    }
    try {
      const data = await mgmtFetch(`/v0/management/model-mappings/test?model=${encodeURIComponent(mappingTestInput.trim())}`)
      setMappingTestResult(JSON.stringify(data, null, 2))
    } catch (e) {
      setMappingTestResult(e instanceof Error ? e.message : String(e))
    }
  }

  useEffect(() => {
    if (mgmtKey) {
      fetchModelMappings()
    }
  }, [mgmtKey])

  if (!mgmtKey) {
    return null
  }

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
      <CardHeader className="pb-4">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-orange-500/10 flex items-center justify-center">
            <ArrowRight className="h-5 w-5 text-orange-500" />
          </div>
          <div className="flex-1">
            <CardTitle className="text-lg">Global Model Mappings</CardTitle>
            <CardDescription>Rewrite model names before routing</CardDescription>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => fetchModelMappings().catch((e) => showToast(String(e), 'error'))}
          >
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Add new mapping form */}
        <div className="flex flex-wrap items-end gap-3 p-3 rounded-lg bg-muted/30 border border-border/50">
          <div className="flex-1 min-w-[150px] space-y-1">
            <label className="text-xs text-muted-foreground">From model</label>
            <input
              type="text"
              className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
              placeholder="gpt-4"
              value={newMapping.from}
              onChange={(e) => setNewMapping({ ...newMapping, from: e.target.value })}
            />
          </div>
          <div className="flex-1 min-w-[150px] space-y-1">
            <label className="text-xs text-muted-foreground">To model</label>
            <input
              type="text"
              className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
              placeholder="claude-3-opus"
              value={newMapping.to}
              onChange={(e) => setNewMapping({ ...newMapping, to: e.target.value })}
            />
          </div>
          <div className="flex-1 min-w-[120px] space-y-1">
            <label className="text-xs text-muted-foreground">Provider (optional)</label>
            <input
              type="text"
              className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
              placeholder="anthropic"
              value={newMapping.provider}
              onChange={(e) => setNewMapping({ ...newMapping, provider: e.target.value })}
            />
          </div>
          <Button
            size="sm"
            variant="default"
            onClick={() => addModelMapping().catch((e) => showToast(String(e), 'error'))}
            className="gap-1"
          >
            <Plus className="h-4 w-4" />
            Add
          </Button>
        </div>

        {/* Mappings list */}
        <div className="max-h-64 overflow-auto rounded-md border border-border/50">
          <table className="w-full text-xs">
            <thead className="bg-muted/40 sticky top-0">
              <tr>
                <th className="px-3 py-2 text-left">From</th>
                <th className="px-3 py-2 text-left"></th>
                <th className="px-3 py-2 text-left">To</th>
                <th className="px-3 py-2 text-left">Provider</th>
                <th className="px-3 py-2 text-left">Actions</th>
              </tr>
            </thead>
            <tbody>
              {modelMappings.length === 0 && (
                <tr>
                  <td className="px-3 py-3 text-muted-foreground" colSpan={5}>
                    No model mappings configured.
                  </td>
                </tr>
              )}
              {modelMappings.map((mapping, index) => (
                <tr key={index} className="border-t border-border/40">
                  {editingMapping === index ? (
                    <>
                      <td className="px-3 py-2">
                        <input
                          type="text"
                          className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                          value={editingMappingValues.from}
                          onChange={(e) => setEditingMappingValues(v => ({ ...v, from: e.target.value }))}
                        />
                      </td>
                      <td className="px-3 py-2 text-muted-foreground">
                        <ArrowRight className="h-4 w-4" />
                      </td>
                      <td className="px-3 py-2">
                        <input
                          type="text"
                          className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                          value={editingMappingValues.to}
                          onChange={(e) => setEditingMappingValues(v => ({ ...v, to: e.target.value }))}
                        />
                      </td>
                      <td className="px-3 py-2">
                        <input
                          type="text"
                          className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
                          value={editingMappingValues.provider}
                          onChange={(e) => setEditingMappingValues(v => ({ ...v, provider: e.target.value }))}
                        />
                      </td>
                      <td className="px-3 py-2">
                        <div className="flex gap-1">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0"
                            onClick={() => {
                              updateModelMapping(index, editingMappingValues)
                                .catch((e) => showToast(String(e), 'error'))
                            }}
                          >
                            <Check className="h-4 w-4 text-green-500" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0"
                            onClick={() => setEditingMapping(null)}
                          >
                            <X className="h-4 w-4" />
                          </Button>
                        </div>
                      </td>
                    </>
                  ) : (
                    <>
                      <td className="px-3 py-2 font-mono">{mapping.from}</td>
                      <td className="px-3 py-2 text-muted-foreground">
                        <ArrowRight className="h-4 w-4" />
                      </td>
                      <td className="px-3 py-2 font-mono">{mapping.to}</td>
                      <td className="px-3 py-2 font-mono text-muted-foreground">
                        {mapping.provider || '-'}
                      </td>
                      <td className="px-3 py-2">
                        <div className="flex gap-1">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0"
                            onClick={() => {
                              setEditingMapping(index)
                              setEditingMappingValues({
                                from: mapping.from || '',
                                to: mapping.to || '',
                                provider: mapping.provider || '',
                              })
                            }}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                            onClick={() => deleteModelMapping(index).catch((e) => showToast(String(e), 'error'))}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </td>
                    </>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Test mapping */}
        <div className="flex flex-wrap items-end gap-3 p-3 rounded-lg bg-muted/30 border border-border/50">
          <div className="flex-1 min-w-[200px] space-y-1">
            <label className="text-xs text-muted-foreground">Test model name</label>
            <input
              type="text"
              className="w-full rounded-md border border-border bg-background/60 px-2 py-1 text-xs font-mono"
              placeholder="Enter model to test mapping"
              value={mappingTestInput}
              onChange={(e) => setMappingTestInput(e.target.value)}
            />
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => testModelMapping().catch((e) => showToast(String(e), 'error'))}
          >
            Test
          </Button>
        </div>
        {mappingTestResult && (
          <pre className="max-h-24 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
            {mappingTestResult}
          </pre>
        )}
      </CardContent>
    </Card>
  )
}
