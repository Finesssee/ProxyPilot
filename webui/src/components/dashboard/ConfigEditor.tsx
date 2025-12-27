import { useState, useEffect } from 'react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Save, RotateCcw, Loader2, FileCode } from 'lucide-react'

export function ConfigEditor() {
    const { mgmtFetch, showToast, status, isMgmtLoading } = useProxyContext()
    const [configYaml, setConfigYaml] = useState('')
    const [isSaving, setIsSaving] = useState(false)
    const isRunning = status?.running ?? false

    const fetchConfig = async () => {
        try {
            const data = await mgmtFetch('/v0/management/config.yaml')
            setConfigYaml(typeof data === 'string' ? data : JSON.stringify(data, null, 2))
        } catch (e) {
            if (!(e instanceof EngineOfflineError)) {
                showToast(e instanceof Error ? e.message : String(e), 'error')
            }
        }
    }

    const saveConfig = async () => {
        setIsSaving(true)
        try {
            await mgmtFetch('/v0/management/config.yaml', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/yaml' },
                body: configYaml,
            })
            showToast('Configuration saved successfully', 'success')
        } catch (e) {
            showToast(e instanceof Error ? e.message : String(e), 'error')
        } finally {
            setIsSaving(false)
        }
    }

    useEffect(() => {
        if (isRunning) {
            fetchConfig()
        }
    }, [isRunning])

    return (
        <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl overflow-hidden">
            <CardHeader className="pb-4 border-b border-border/50 bg-gradient-to-r from-purple-500/5 to-transparent">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="h-10 w-10 rounded-xl bg-purple-500/10 flex items-center justify-center">
                            <FileCode className="h-5 w-5 text-purple-500" />
                        </div>
                        <div>
                            <CardTitle className="text-lg">Config Editor</CardTitle>
                            <CardDescription>Edit config.yaml directly</CardDescription>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={fetchConfig}
                            disabled={!isRunning || isMgmtLoading}
                            className="gap-1.5 text-xs"
                        >
                            <RotateCcw className="h-3.5 w-3.5" />
                            Reset
                        </Button>
                        <Button
                            size="sm"
                            onClick={saveConfig}
                            disabled={!isRunning || isSaving || isMgmtLoading}
                            className="gap-1.5 text-xs bg-purple-600 hover:bg-purple-700 text-white"
                        >
                            {isSaving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
                            Save Changes
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent className="p-0">
                {!isRunning ? (
                    <div className="py-12 text-center text-muted-foreground">
                        <p className="text-sm uppercase tracking-widest font-mono">⚠️ Engine Offline</p>
                        <p className="text-xs mt-1">Start the proxy engine to edit configuration</p>
                    </div>
                ) : (
                    <div className="relative">
                        <textarea
                            value={configYaml}
                            onChange={(e) => setConfigYaml(e.target.value)}
                            spellCheck={false}
                            className="w-full h-[500px] p-6 bg-[var(--bg-void)] text-[var(--text-primary)] font-mono text-sm resize-none focus:outline-none selection:bg-purple-500/30"
                            style={{
                                fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                                lineHeight: '1.6',
                            }}
                        />
                        {/* Decorative grid background for the editor */}
                        <div
                            className="absolute inset-0 pointer-events-none opacity-[0.03]"
                            style={{
                                backgroundImage: `linear-gradient(var(--border-subtle) 1px, transparent 1px), linear-gradient(90deg, var(--border-subtle) 1px, transparent 1px)`,
                                backgroundSize: '20px 20px'
                            }}
                        />
                    </div>
                )}
            </CardContent>
        </Card>
    )
}
