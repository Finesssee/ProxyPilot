import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Key, ExternalLink, Info } from 'lucide-react'
import { useProxyContext } from '@/hooks/useProxyContext'

const providers = [
  { id: 'antigravity', name: 'Antigravity', color: 'from-purple-500 to-indigo-600' },
  { id: 'gemini-cli', name: 'Gemini CLI', color: 'from-blue-500 to-cyan-500' },
  { id: 'codex', name: 'Codex', color: 'from-green-500 to-emerald-600' },
  { id: 'claude', name: 'Claude', color: 'from-orange-500 to-amber-500' },
  { id: 'qwen', name: 'Qwen', color: 'from-pink-500 to-rose-500' },
  { id: 'iflow', name: 'iFlow', color: 'from-teal-500 to-cyan-600' },
]

export function ProviderLogins() {
  const { loading, setLoading, showToast } = useProxyContext()

  const handleOAuth = async (provider: string) => {
    setLoading(`oauth-${provider}`)
    try {
      if (window.pp_oauth) {
        await window.pp_oauth(provider)
        showToast(`Opening ${provider} login...`, 'success')
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    } finally {
      setLoading(null)
    }
  }

  return (
    <Card className="backdrop-blur-sm bg-card/60 border-border/50 shadow-xl">
      <CardHeader className="pb-4">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-purple-500/10 flex items-center justify-center">
            <Key className="h-5 w-5 text-purple-500" />
          </div>
          <div>
            <CardTitle className="text-lg">Provider Logins</CardTitle>
            <CardDescription>Authenticate with AI providers</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-3">
          {providers.map((provider) => (
            <Button
              key={provider.id}
              variant="outline"
              className={`relative overflow-hidden transition-all duration-300 hover:border-primary/50 group h-12 bg-background/50`}
              onClick={() => handleOAuth(provider.id)}
              disabled={loading === `oauth-${provider.id}`}
            >
              <div className={`absolute inset-0 bg-gradient-to-r ${provider.color} opacity-0 group-hover:opacity-10 transition-opacity duration-300`} />
              <span className="relative flex items-center gap-2 font-medium">
                <ExternalLink className="h-4 w-4 opacity-50 group-hover:opacity-100 transition-opacity" />
                {provider.name}
              </span>
            </Button>
          ))}
        </div>

        <div className="flex items-start gap-3 rounded-lg bg-blue-500/5 border border-blue-500/10 p-4">
          <Info className="h-5 w-5 text-blue-500 mt-0.5 flex-shrink-0" />
          <p className="text-sm text-muted-foreground leading-relaxed">
            Click a provider to open their login flow in your browser. Authentication tokens will be saved locally for ProxyPilot to use.
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
