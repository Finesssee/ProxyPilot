import { useState, useEffect } from 'react'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Lock, LockOpen, ChevronDown, RefreshCw } from 'lucide-react'
import { useProxyContext, EngineOfflineError } from '@/hooks/useProxyContext'
import { cn } from '@/lib/utils'

// Provider configuration with ProxyPilot Aviation theme
const providers = [
  { id: 'claude', name: 'Claude', color: 'oklch(0.60 0.15 35)', icon: '🤖' },
  { id: 'gemini', name: 'Gemini', color: 'oklch(0.55 0.18 250)', icon: '✨' },
  { id: 'codex', name: 'Codex', color: 'oklch(0.60 0.16 145)', icon: '💻' },
  { id: 'qwen', name: 'Qwen', color: 'oklch(0.60 0.14 280)', icon: '🔮' },
  { id: 'anthropic', name: 'Anthropic', color: 'oklch(0.55 0.14 50)', icon: '🅰️' },
  { id: 'iflow', name: 'iFlow', color: 'oklch(0.55 0.12 220)', icon: '🌊' },
  { id: 'antigravity', name: 'Antigravity', color: 'oklch(0.65 0.20 320)', icon: '🚀' },
] as const

type ProviderId = (typeof providers)[number]['id']

// Signal bar heights (20%, 40%, 60%, 80%, 100%)
const signalBarHeights = [20, 40, 60, 80, 100]

interface SignalBarsProps {
  isConnected: boolean
  color: string
}

function SignalBars({ isConnected, color }: SignalBarsProps) {
  return (
    <div className="flex items-end justify-center gap-[3px] h-5">
      {signalBarHeights.map((height, index) => (
        <div
          key={index}
          className={cn(
            'w-[5px] rounded-sm transition-all duration-300',
            isConnected && 'animate-signal-pulse'
          )}
          style={{
            height: `${height}%`,
            background: isConnected ? color : 'var(--border-subtle)',
            boxShadow: isConnected ? `0 0 6px ${color}` : 'none',
            animationDelay: isConnected ? `${index * 100}ms` : '0ms',
          }}
        />
      ))}
    </div>
  )
}

interface ProviderCardProps {
  provider: (typeof providers)[number]
  isAuthenticated: boolean
  isLoading: boolean
  isDisabled: boolean
  onClick: () => void
  index: number
}

function ProviderCard({ provider, isAuthenticated, isLoading, isDisabled, onClick, index }: ProviderCardProps) {
  const delayClass = `delay-${(index % 6) * 100}`

  return (
    <div
      className={cn(
        'group relative flex flex-col items-center p-5',
        'bg-[var(--bg-panel)] border border-[var(--border-subtle)] rounded-lg',
        'transition-all duration-300 ease-out',
        'hover:border-transparent',
        'animate-fade-in-up',
        delayClass,
        isLoading && 'animate-connecting-pulse',
        isDisabled && 'opacity-50 grayscale pointer-events-none'
      )}
      style={{
        ['--provider-color' as string]: provider.color,
      }}
    >
      {/* Ambient glow effect on hover */}
      <div
        className="absolute inset-0 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
        style={{
          boxShadow: `0 0 30px color-mix(in oklch, ${provider.color} 25%, transparent), inset 0 0 20px color-mix(in oklch, ${provider.color} 8%, transparent)`,
        }}
      />

      {/* Animated border on hover */}
      <div
        className="absolute inset-0 rounded-lg border-2 opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
        style={{
          borderColor: provider.color,
          boxShadow: `inset 0 0 15px color-mix(in oklch, ${provider.color} 15%, transparent)`,
        }}
      />

      {/* Glowing Icon Container */}
      <div
        className={cn(
          'relative w-14 h-14 rounded-xl flex items-center justify-center text-2xl',
          'transition-all duration-300',
          'group-hover:scale-110'
        )}
        style={{
          background: `linear-gradient(135deg, color-mix(in oklch, ${provider.color} 20%, transparent), color-mix(in oklch, ${provider.color} 10%, transparent))`,
          border: `1px solid color-mix(in oklch, ${provider.color} 40%, transparent)`,
          boxShadow: isAuthenticated
            ? `0 0 20px color-mix(in oklch, ${provider.color} 40%, transparent), inset 0 0 10px color-mix(in oklch, ${provider.color} 15%, transparent)`
            : `0 0 10px color-mix(in oklch, ${provider.color} 20%, transparent)`,
        }}
      >
        {/* Icon glow pulse when connected */}
        {isAuthenticated && (
          <div
            className="absolute inset-0 rounded-xl animate-pulse-glow"
            style={{
              boxShadow: `0 0 25px ${provider.color}`,
            }}
          />
        )}
        <span className="relative z-10">{provider.icon}</span>
      </div>

      {/* Provider Name */}
      <div
        className="mt-4 font-bold uppercase tracking-[0.15em] text-[var(--text-primary)] text-center text-xs"
        style={{ fontFamily: 'var(--font-display)' }}
      >
        {provider.name}
      </div>

      {/* Signal Strength Bars */}
      <div className="mt-4">
        <SignalBars isConnected={isAuthenticated} color={provider.color} />
      </div>

      {/* Status Text */}
      <div
        className={cn(
          'mt-2 text-[0.65rem] uppercase tracking-wider',
          isAuthenticated ? 'text-[var(--accent-glow)]' : 'text-[var(--text-muted)]'
        )}
        style={{ fontFamily: 'var(--font-mono)' }}
      >
        {isAuthenticated ? 'Connected' : 'Offline'}
      </div>

      {/* Action Button */}
      <button
        onClick={onClick}
        disabled={isLoading || isDisabled}
        className={cn(
          'mt-4 px-5 py-1.5 rounded text-[0.7rem] uppercase tracking-wider font-semibold',
          'border transition-all duration-200',
          'focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg-panel)]',
          'disabled:opacity-50 disabled:cursor-wait',
          isAuthenticated
            ? 'bg-transparent hover:bg-[var(--bg-elevated)]'
            : 'hover:scale-105'
        )}
        style={{
          fontFamily: 'var(--font-mono)',
          borderColor: provider.color,
          color: provider.color,
          boxShadow: `0 0 10px color-mix(in oklch, ${provider.color} 30%, transparent)`,
        }}
      >
        {isLoading ? (
          <span className="flex items-center gap-1.5">
            <span className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
            Linking
          </span>
        ) : isAuthenticated ? (
          'Relink'
        ) : (
          'Login'
        )}
      </button>
    </div>
  )
}

function ProviderSkeleton() {
  return (
    <div className="flex flex-col items-center p-5 bg-[var(--bg-panel)] border border-[var(--border-subtle)] rounded-lg animate-pulse">
      <div className="w-14 h-14 rounded-xl bg-[var(--bg-elevated)]" />
      <div className="mt-4 h-3 w-16 bg-[var(--bg-elevated)] rounded" />
      <div className="mt-4 h-5 w-12 bg-[var(--bg-elevated)] rounded" />
      <div className="mt-2 h-2 w-10 bg-[var(--bg-elevated)] rounded" />
      <div className="mt-4 h-8 w-20 bg-[var(--bg-elevated)] rounded" />
    </div>
  )
}

interface AuthFileInfo {
  id: string
  provider?: string
  type?: string
  email?: string
  label?: string
  status?: string
}

export function ProviderLogins() {
  const { loading, setLoading, showToast, authFiles, status, isMgmtLoading, mgmtKey, mgmtFetch } = useProxyContext()
  const [privateOAuth, setPrivateOAuth] = useState(false)
  const [showConfig, setShowConfig] = useState(false)
  const [authFileList, setAuthFileList] = useState<AuthFileInfo[]>([])
  const [mgmtConfig, setMgmtConfig] = useState<any>(null)

  const isRunning = status?.running ?? false

  // Auth status based on file names
  const authStatus: Record<ProviderId, boolean> = {
    claude: authFiles.some(f => f.toLowerCase().includes('claude')),
    gemini: authFiles.some(f => f.toLowerCase().includes('gemini')),
    codex: authFiles.some(f => f.toLowerCase().includes('codex')),
    qwen: authFiles.some(f => f.toLowerCase().includes('qwen')),
    anthropic: authFiles.some(f => f.toLowerCase().includes('anthropic')),
    iflow: authFiles.some(f => f.toLowerCase().includes('iflow')),
    antigravity: authFiles.some(f => f.toLowerCase().includes('antigravity')),
  }

  // Load OAuth private setting on mount
  useEffect(() => {
    ;(async () => {
      try {
        if (window.pp_get_oauth_private) {
          const priv = await window.pp_get_oauth_private()
          setPrivateOAuth(priv)
        }
      } catch (e) {
        console.error('OAuth private error:', e)
      }
    })()
  }, [])

  // Load config when running
  useEffect(() => {
    if (!mgmtKey || !isRunning) return
    ;(async () => {
      try {
        const cfg = await mgmtFetch('/v0/management/config')
        setMgmtConfig(cfg)
        const res = await mgmtFetch('/v0/management/auth-files')
        setAuthFileList(res.files || [])
      } catch (e) {
        if (!(e instanceof EngineOfflineError)) {
          console.error('Config load error:', e)
        }
      }
    })()
  }, [mgmtKey, isRunning])

  const handlePrivateOAuthChange = async (checked: boolean) => {
    try {
      if (window.pp_set_oauth_private) {
        await window.pp_set_oauth_private(checked)
        setPrivateOAuth(checked)
        showToast('Encryption mode updated', 'success')
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const handleOAuth = async (provider: string) => {
    setLoading(`oauth-${provider}`)
    try {
      if (window.pp_oauth) {
        await window.pp_oauth(provider)
        showToast(`Establishing uplink to ${provider}...`, 'success')
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    } finally {
      setLoading(null)
    }
  }

  const toggleDebug = async () => {
    try {
      const cur = await mgmtFetch('/v0/management/debug')
      await mgmtFetch('/v0/management/debug', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value: !cur.value }),
      })
      const cfg = await mgmtFetch('/v0/management/config')
      setMgmtConfig(cfg)
    } catch (e) {
      console.error(e)
    }
  }

  const resetCooldown = async (authId?: string) => {
    try {
      await mgmtFetch('/v0/management/auth/reset-cooldown', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(authId ? { auth_id: authId } : {}),
      })
      const res = await mgmtFetch('/v0/management/auth-files')
      setAuthFileList(res.files || [])
      showToast('Cooldowns reset', 'success')
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  const debugOn = !!mgmtConfig?.debug

  return (
    <div
      className={cn(
        'bg-[var(--bg-panel)] border border-[var(--border-subtle)] rounded-lg',
        'shadow-lg overflow-hidden'
      )}
    >
      {/* Section Header with Communication Array styling */}
      <div
        className={cn(
          'flex items-center justify-between px-5 py-3',
          'border-b border-[var(--border-subtle)]',
          'bg-gradient-to-r from-[var(--bg-elevated)] to-transparent'
        )}
      >
        <div className="flex items-center gap-2">
          <span
            className="text-[var(--accent-glow)] text-sm tracking-wider"
            style={{ fontFamily: 'var(--font-mono)' }}
          >
            ::
          </span>
          <span
            className="text-[var(--text-primary)] text-sm font-bold uppercase tracking-[0.15em]"
            style={{ fontFamily: 'var(--font-display)' }}
          >
            Providers
          </span>
        </div>

        {/* Private OAuth Toggle - Secure/Encrypted indicator */}
        <div className="flex items-center gap-3">
          <div
            className={cn(
              'flex items-center gap-2 px-3 py-1 rounded-full border transition-all duration-200',
              privateOAuth
                ? 'border-[var(--accent-glow)] bg-[color-mix(in_oklch,var(--accent-glow)_10%,transparent)]'
                : 'border-[var(--border-subtle)] bg-transparent'
            )}
          >
            {privateOAuth ? (
              <Lock className="h-3.5 w-3.5 text-[var(--accent-glow)]" />
            ) : (
              <LockOpen className="h-3.5 w-3.5 text-[var(--text-muted)]" />
            )}
            <Label
              htmlFor="private-oauth-providers"
              className={cn(
                'text-[0.65rem] cursor-pointer uppercase tracking-wider',
                privateOAuth ? 'text-[var(--accent-glow)]' : 'text-[var(--text-muted)]'
              )}
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              {privateOAuth ? 'Encrypted' : 'Private'}
            </Label>
            <Switch
              id="private-oauth-providers"
              checked={privateOAuth}
              onCheckedChange={handlePrivateOAuthChange}
              className="scale-75 data-[state=checked]:bg-[var(--accent-glow)]"
            />
          </div>
        </div>
      </div>

      {/* Provider Grid - Satellite Uplinks */}
      <div className="p-5">
        {!isRunning && (
          <div className="mb-6 p-4 rounded-lg border border-yellow-500/30 bg-yellow-500/5 text-yellow-500 text-xs text-center uppercase tracking-widest font-mono">
            ⚠️ Please start the proxy engine to manage providers
          </div>
        )}

        <div
          className="grid gap-5"
          style={{
            gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
          }}
        >
          {isMgmtLoading && authFiles.length === 0 ? (
            Array.from({ length: 6 }).map((_, i) => <ProviderSkeleton key={i} />)
          ) : (
            providers.map((provider, index) => (
              <ProviderCard
                key={provider.id}
                index={index}
                provider={provider}
                isAuthenticated={authStatus[provider.id]}
                isLoading={loading === `oauth-${provider.id}`}
                isDisabled={!isRunning}
                onClick={() => handleOAuth(provider.id)}
              />
            ))
          )}
        </div>

        {/* Consolidated Configuration Section */}
        {mgmtKey && isRunning && (
          <div className="mt-6 pt-5 border-t border-[var(--border-subtle)]">
            <button
              onClick={() => setShowConfig(!showConfig)}
              className="flex items-center justify-between w-full text-left group"
            >
              <span
                className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] group-hover:text-[var(--text-primary)] transition-colors"
                style={{ fontFamily: 'var(--font-mono)' }}
              >
                Configuration & Accounts
              </span>
              <ChevronDown className={cn(
                'h-4 w-4 text-[var(--text-muted)] transition-transform',
                showConfig && 'rotate-180'
              )} />
            </button>

            {showConfig && (
              <div className="mt-4 space-y-4 animate-in slide-in-from-top-2 duration-200">
                {/* Quick Settings Row */}
                <div className="flex flex-wrap items-center gap-3">
                  <button
                    onClick={() => toggleDebug().catch(console.error)}
                    className={cn(
                      'px-3 py-1.5 rounded-md text-xs font-semibold uppercase tracking-wider border transition-all',
                      debugOn
                        ? 'border-[var(--accent-primary)] bg-[var(--accent-primary)]/10 text-[var(--accent-primary)] shadow-[0_0_10px_var(--accent-primary)/20]'
                        : 'border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                    )}
                    style={{ fontFamily: 'var(--font-mono)' }}
                  >
                    Debug: {debugOn ? 'ON' : 'OFF'}
                  </button>

                  <button
                    onClick={() => resetCooldown().catch(console.error)}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold uppercase tracking-wider border border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                    style={{ fontFamily: 'var(--font-mono)' }}
                  >
                    <RefreshCw className="h-3 w-3" />
                    Reset Cooldowns
                  </button>
                </div>

                {/* Auth Files Table */}
                {authFileList.length > 0 && (
                  <div className="rounded-lg border border-[var(--border-subtle)] overflow-hidden">
                    <table className="w-full text-xs">
                      <thead className="bg-[var(--bg-elevated)]">
                        <tr>
                          <th className="px-3 py-2 text-left text-[var(--text-muted)] font-semibold uppercase tracking-wider" style={{ fontFamily: 'var(--font-mono)' }}>Provider</th>
                          <th className="px-3 py-2 text-left text-[var(--text-muted)] font-semibold uppercase tracking-wider" style={{ fontFamily: 'var(--font-mono)' }}>Account</th>
                          <th className="px-3 py-2 text-left text-[var(--text-muted)] font-semibold uppercase tracking-wider" style={{ fontFamily: 'var(--font-mono)' }}>Status</th>
                        </tr>
                      </thead>
                      <tbody>
                        {authFileList.map((f) => (
                          <tr key={f.id} className="border-t border-[var(--border-subtle)]">
                            <td className="px-3 py-2 font-mono text-[var(--text-primary)]">
                              {f.provider || f.type || '—'}
                            </td>
                            <td className="px-3 py-2 text-[var(--text-secondary)]">
                              {f.email || f.label || '—'}
                            </td>
                            <td className="px-3 py-2">
                              <span className={cn(
                                'inline-flex px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider',
                                f.status === 'active' || f.status === 'ok'
                                  ? 'bg-[var(--status-online)]/15 text-[var(--status-online)]'
                                  : 'bg-[var(--text-muted)]/15 text-[var(--text-muted)]'
                              )} style={{ fontFamily: 'var(--font-mono)' }}>
                                {f.status || 'unknown'}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Custom keyframe styles */}
      <style>{`
        @keyframes pulse-glow {
          0%, 100% { opacity: 0.4; }
          50% { opacity: 0.8; }
        }

        .animate-pulse-glow {
          animation: pulse-glow 2s ease-in-out infinite;
        }
      `}</style>
    </div>
  )
}
