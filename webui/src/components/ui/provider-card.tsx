import { cn } from '@/lib/utils'

interface ProviderCardProps {
  provider: 'claude' | 'gemini' | 'codex' | 'qwen' | 'anthropic' | 'openai'
  status: 'authenticated' | 'expired' | 'none'
  lastAuth?: string
  onLogin: () => void
  onLogout?: () => void
  disabled?: boolean
}

const providerConfig = {
  claude: {
    name: 'Claude',
    color: 'oklch(0.60 0.15 35)',
    icon: '🤖',
  },
  gemini: {
    name: 'Gemini',
    color: 'oklch(0.55 0.18 250)',
    icon: '✨',
  },
  codex: {
    name: 'Codex',
    color: 'oklch(0.60 0.16 145)',
    icon: '💻',
  },
  qwen: {
    name: 'Qwen',
    color: 'oklch(0.60 0.14 280)',
    icon: '🔮',
  },
  anthropic: {
    name: 'Anthropic',
    color: 'oklch(0.55 0.14 50)',
    icon: '🅰️',
  },
  openai: {
    name: 'OpenAI',
    color: 'oklch(0.55 0.12 145)',
    icon: '⚡',
  },
} as const

const statusConfig = {
  authenticated: {
    label: 'Authenticated',
    dotClass: 'bg-[var(--status-online)]',
    textClass: 'text-[var(--status-online)]',
  },
  expired: {
    label: 'Expired',
    dotClass: 'bg-[var(--status-warning)]',
    textClass: 'text-[var(--status-warning)]',
  },
  none: {
    label: 'Not Connected',
    dotClass: 'bg-[var(--text-muted)]',
    textClass: 'text-[var(--text-muted)]',
  },
} as const

export function ProviderCard({
  provider,
  status,
  lastAuth,
  onLogin,
  onLogout,
  disabled = false,
}: ProviderCardProps) {
  const config = providerConfig[provider]
  const statusInfo = statusConfig[status]

  return (
    <div
      className={cn(
        'group relative w-[160px] p-4',
        'bg-[var(--bg-panel)] border border-[var(--border-subtle)] rounded-md',
        'transition-all duration-150 ease-out',
        disabled && 'opacity-50 pointer-events-none'
      )}
      style={{
        ['--provider-color' as string]: config.color,
      }}
    >
      {/* Hover glow effect */}
      <div
        className="absolute inset-0 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-150 pointer-events-none"
        style={{
          boxShadow: `0 0 20px color-mix(in oklch, ${config.color} 20%, transparent)`,
        }}
      />

      {/* Hover border */}
      <div
        className="absolute inset-0 rounded-md border opacity-0 group-hover:opacity-100 transition-opacity duration-150 pointer-events-none"
        style={{
          borderColor: config.color,
        }}
      />

      {/* Icon container */}
      <div
        className="w-10 h-10 rounded-lg flex items-center justify-center text-xl"
        style={{
          background: `color-mix(in oklch, ${config.color} 15%, transparent)`,
          border: `1px solid color-mix(in oklch, ${config.color} 30%, transparent)`,
        }}
      >
        {config.icon}
      </div>

      {/* Provider name */}
      <div
        className="mt-3 font-[var(--font-display)] font-semibold uppercase tracking-[0.05em] text-[var(--text-primary)]"
        style={{ fontFamily: 'var(--font-display)' }}
      >
        {config.name}
      </div>

      {/* Status indicator */}
      <div
        className={cn(
          'mt-3 text-[0.7rem] font-mono flex items-center gap-1.5',
          statusInfo.textClass
        )}
        style={{ fontFamily: 'var(--font-mono)' }}
      >
        <span className={cn('w-1.5 h-1.5 rounded-full', statusInfo.dotClass)} />
        {statusInfo.label}
      </div>

      {/* Last auth time */}
      {lastAuth && (
        <div
          className="mt-1 text-[0.65rem] text-[var(--text-muted)]"
          style={{ fontFamily: 'var(--font-mono)' }}
        >
          Last: {lastAuth}
        </div>
      )}

      {/* Action button */}
      <div className="mt-4">
        {status === 'authenticated' && onLogout ? (
          <button
            onClick={onLogout}
            disabled={disabled}
            className={cn(
              'w-full px-3 py-1.5 text-xs font-medium uppercase tracking-wide',
              'bg-transparent border border-[var(--border-subtle)] rounded',
              'text-[var(--text-secondary)]',
              'hover:border-[var(--text-muted)] hover:text-[var(--text-primary)]',
              'transition-all duration-150',
              'disabled:opacity-50 disabled:cursor-not-allowed'
            )}
          >
            Logout
          </button>
        ) : (
          <button
            onClick={onLogin}
            disabled={disabled}
            className="w-full px-3 py-1.5 text-xs font-medium uppercase tracking-wide rounded transition-all duration-150 disabled:opacity-50 disabled:cursor-not-allowed"
            style={{
              background: `color-mix(in oklch, ${config.color} 20%, transparent)`,
              border: `1px solid color-mix(in oklch, ${config.color} 40%, transparent)`,
              color: config.color,
            }}
          >
            Login
          </button>
        )}
      </div>
    </div>
  )
}
