import { Terminal, Key, GitBranch, Database, ScrollText, Activity, BarChart3, type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface IconRailItem {
  id: string
  icon: LucideIcon
  label: string
  color?: string
  shortcut?: string
}

interface IconRailProps {
  items: IconRailItem[]
  activeId: string
  onSelect: (id: string) => void
}

export const navigationItems: IconRailItem[] = [
  { id: 'command', icon: Terminal, label: 'Command', color: 'var(--accent-primary)', shortcut: 'Ctrl+1' },
  { id: 'providers', icon: Key, label: 'Providers', color: 'var(--accent-glow)', shortcut: 'Ctrl+2' },
  { id: 'routing', icon: GitBranch, label: 'Routing', color: 'var(--status-processing)', shortcut: 'Ctrl+3' },
  { id: 'memory', icon: Database, label: 'Memory', color: 'var(--accent-secondary)', shortcut: 'Ctrl+4' },
  { id: 'logs', icon: ScrollText, label: 'Logs', color: 'var(--text-secondary)', shortcut: 'Ctrl+5' },
  { id: 'requests', icon: Activity, label: 'Monitor', color: 'var(--accent-glow)', shortcut: 'Ctrl+6' },
  { id: 'analytics', icon: BarChart3, label: 'Analytics', color: 'var(--accent-primary)', shortcut: 'Ctrl+7' },
]

export function IconRail({ items, activeId, onSelect }: IconRailProps) {
  return (
    <nav
      className="relative flex flex-col items-center py-3 gap-1"
      style={{
        width: '64px',
        minWidth: '64px',
        height: '100%',
        backgroundColor: 'var(--bg-panel)',
        borderRight: '1px solid var(--border-subtle)',
      }}
    >
      {/* Aviation grid background pattern */}
      <div
        className="absolute inset-0 pointer-events-none opacity-30"
        style={{
          backgroundImage: `
            linear-gradient(to right, var(--border-subtle) 1px, transparent 1px),
            linear-gradient(to bottom, var(--border-subtle) 1px, transparent 1px)
          `,
          backgroundSize: '8px 8px',
        }}
      />

      {/* Runway center line (decorative) */}
      <div
        className="absolute left-1/2 -translate-x-1/2 pointer-events-none opacity-20"
        style={{
          top: '12px',
          bottom: '12px',
          width: '2px',
          background: `repeating-linear-gradient(
            to bottom,
            var(--border-default) 0px,
            var(--border-default) 8px,
            transparent 8px,
            transparent 16px
          )`,
        }}
      />

      {items.map((item) => {
        const isActive = item.id === activeId
        const Icon = item.icon
        const itemColor = item.color || 'var(--text-muted)'

        return (
          <div key={item.id} className="relative group z-10">
            <button
              onClick={() => onSelect(item.id)}
              className={cn(
                'relative flex items-center justify-center transition-all duration-300 ease-out',
                'focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2',
                'group-hover:scale-105'
              )}
              style={{
                width: '48px',
                height: '48px',
                borderRadius: '8px',
                backgroundColor: isActive
                  ? `color-mix(in srgb, ${itemColor} 15%, transparent)`
                  : 'transparent',
                boxShadow: isActive
                  ? `inset 0 0 12px color-mix(in srgb, ${itemColor} 20%, transparent)`
                  : 'none',
              }}
              aria-label={item.label}
              aria-current={isActive ? 'page' : undefined}
            >
              {/* Runway-style active indicator - left vertical strip */}
              {isActive && (
                <span
                  className="absolute left-0 top-0 bottom-0 rounded-r-sm animate-glow-pulse"
                  style={{
                    width: '3px',
                    backgroundColor: itemColor,
                    boxShadow: `0 0 8px ${itemColor}, 0 0 16px ${itemColor}`,
                  }}
                />
              )}

              {/* Icon with glow halo when active or hovered */}
              <div
                className={cn(
                  'relative flex items-center justify-center transition-all duration-300',
                  isActive && 'animate-signal-pulse',
                  'group-hover:scale-110'
                )}
                style={{
                  filter: isActive
                    ? `drop-shadow(0 0 8px ${itemColor})`
                    : 'none',
                }}
              >
                <Icon
                  size={20}
                  className="transition-all duration-300"
                  style={{
                    color: isActive ? itemColor : 'var(--text-muted)',
                    filter: !isActive ? 'drop-shadow(0 0 0px transparent)' : undefined,
                  }}
                />
              </div>

              {/* Hover glow effect (non-active items) */}
              <span
                className={cn(
                  'absolute inset-0 rounded-lg transition-all duration-300',
                  'opacity-0 group-hover:opacity-100',
                  isActive && 'group-hover:opacity-0'
                )}
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  boxShadow: `0 0 15px color-mix(in srgb, ${itemColor} 15%, transparent), inset 0 0 8px color-mix(in srgb, ${itemColor} 10%, transparent)`,
                }}
              />
            </button>

            {/* Instrument Readout Tooltip */}
            <div
              className={cn(
                'absolute left-full top-1/2 -translate-y-1/2 ml-3',
                'opacity-0 invisible group-hover:opacity-100 group-hover:visible',
                'transition-all duration-200 ease-out',
                'pointer-events-none z-50'
              )}
            >
              <div
                className="relative px-3 py-2 rounded"
                style={{
                  backgroundColor: 'var(--bg-void)',
                  border: '1px solid var(--accent-glow)',
                  boxShadow: `0 0 12px color-mix(in srgb, var(--accent-glow) 30%, transparent), 0 4px 16px rgba(0, 0, 0, 0.4)`,
                }}
              >
                {/* Tooltip content - Instrument readout style */}
                <div className="flex flex-col gap-0.5">
                  {/* Label with arrow prefix */}
                  <div
                    className="flex items-center gap-1.5 whitespace-nowrap"
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '11px',
                      fontWeight: 600,
                      letterSpacing: '0.05em',
                      color: 'var(--text-primary)',
                    }}
                  >
                    <span style={{ color: itemColor }}>▸</span>
                    <span>{item.label.toUpperCase()}</span>
                  </div>

                  {/* Keyboard shortcut */}
                  {item.shortcut && (
                    <div
                      className="whitespace-nowrap"
                      style={{
                        fontFamily: 'var(--font-mono)',
                        fontSize: '9px',
                        color: 'var(--text-muted)',
                        letterSpacing: '0.02em',
                        paddingLeft: '14px',
                      }}
                    >
                      {item.shortcut}
                    </div>
                  )}
                </div>

                {/* Tooltip pointer (left-pointing triangle) */}
                <span
                  className="absolute right-full top-1/2 -translate-y-1/2"
                  style={{
                    width: 0,
                    height: 0,
                    borderTop: '6px solid transparent',
                    borderBottom: '6px solid transparent',
                    borderRight: '6px solid var(--accent-glow)',
                  }}
                />

                {/* Inner pointer for fill */}
                <span
                  className="absolute right-full top-1/2 -translate-y-1/2"
                  style={{
                    width: 0,
                    height: 0,
                    marginRight: '-1px',
                    borderTop: '5px solid transparent',
                    borderBottom: '5px solid transparent',
                    borderRight: '5px solid var(--bg-void)',
                  }}
                />
              </div>
            </div>
          </div>
        )
      })}
    </nav>
  )
}

export default IconRail
