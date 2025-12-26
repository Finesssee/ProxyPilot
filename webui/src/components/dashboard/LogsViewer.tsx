import { useState } from 'react'
import { useProxyContext } from '@/hooks/useProxyContext'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Copy, FileText } from 'lucide-react'

export function LogsViewer() {
  const { mgmtFetch, showToast } = useProxyContext()
  const [diagnostics, setDiagnostics] = useState('')
  const [logText, setLogText] = useState('')
  const [activeLogType, setActiveLogType] = useState<'stdout' | 'stderr'>('stdout')

  const loadDiagnostics = async () => {
    try {
      const res = await mgmtFetch('/v0/management/proxypilot/diagnostics?lines=120')
      setDiagnostics(res.text || '')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const copyDiagnostics = async () => {
    if (!diagnostics) {
      showToast('No diagnostics to copy', 'error')
      return
    }
    try {
      await navigator.clipboard.writeText(diagnostics)
      showToast('Copied diagnostics to clipboard', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const tailLogs = async (kind: 'stdout' | 'stderr') => {
    try {
      setActiveLogType(kind)
      const res = await mgmtFetch(`/v0/management/proxypilot/logs/tail?file=${kind}&lines=200`)
      setLogText((res.lines || []).join('\n'))
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* Diagnostics Card */}
      <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
        <CardHeader className="pb-4">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-blue-500/10 flex items-center justify-center">
              <FileText className="h-5 w-5 text-blue-500" />
            </div>
            <div>
              <CardTitle className="text-lg">Diagnostics</CardTitle>
              <CardDescription>Engine snapshot</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={loadDiagnostics}>
              Load
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={copyDiagnostics}
              disabled={!diagnostics}
              className="gap-2"
            >
              <Copy className="h-4 w-4" />
              Copy
            </Button>
          </div>
          <pre className="max-h-48 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
            {diagnostics || 'No diagnostics loaded.'}
          </pre>
        </CardContent>
      </Card>

      {/* Logs Tail Card */}
      <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
        <CardHeader className="pb-4">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-xl bg-green-500/10 flex items-center justify-center">
              <FileText className="h-5 w-5 text-green-500" />
            </div>
            <div>
              <CardTitle className="text-lg">Logs (tail)</CardTitle>
              <CardDescription>Engine stdout/stderr</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Button
              size="sm"
              variant={activeLogType === 'stdout' ? 'default' : 'outline'}
              onClick={() => tailLogs('stdout')}
            >
              Stdout
            </Button>
            <Button
              size="sm"
              variant={activeLogType === 'stderr' ? 'default' : 'outline'}
              onClick={() => tailLogs('stderr')}
            >
              Stderr
            </Button>
          </div>
          <pre className="max-h-48 overflow-auto rounded-md border border-border/50 bg-muted/30 p-2 text-xs">
            {logText || 'No logs loaded.'}
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
