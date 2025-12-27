import { Settings } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'
import { ThemeToggle } from '../ui/theme-toggle'
import { SettingsPanel } from './SettingsPanel'
import { cn } from '@/lib/utils'

interface HeaderProps {
  isRunning: boolean
  port?: number
  version?: string
}

/**
 * Paper Airplane Logo SVG
 * Geometric design matching the ProxyPilot app icon
 */
function PaperAirplaneLogo({ className, isHovered }: { className?: string; isHovered?: boolean }) {
  return (
    <svg
      viewBox="0 0 32 32"
      className={cn('w-8 h-8', className)}
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      {/* Main airplane body - folded paper look */}
      <path
        d="M4 16L28 4L20 28L16 18L4 16Z"
        fill="currentColor"
        className="opacity-90"
      />
      {/* Wing fold line */}
      <path
        d="M28 4L16 18"
        stroke="var(--bg-panel)"
        strokeWidth="1.5"
        strokeLinecap="round"
        className="opacity-60"
      />
      {/* Inner fold detail */}
      <path
        d="M4 16L16 18L20 28"
        stroke="var(--bg-panel)"
        strokeWidth="1"
        strokeLinecap="round"
        className="opacity-40"
      />
      {/* Glow effect when running */}
      {isHovered && (
        <circle
          cx="16"
          cy="16"
          r="14"
          fill="none"
          stroke="currentColor"
          strokeWidth="1"
          className="opacity-30 animate-radar-pulse"
        />
      )}
    </svg>
  )
}

/**
 * Animated Flight Path Line
 * Draws on hover to simulate navigation route
 */
function FlightPathLine({ isVisible }: { isVisible: boolean }) {
  return (
    <svg
      viewBox="0 0 200 12"
      className={cn(
        'absolute left-0 bottom-0 w-full h-3 pointer-events-none',
        'transition-opacity duration-300',
        isVisible ? 'opacity-100' : 'opacity-0'
      )}
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id="flightPathGradient" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="var(--accent-primary)" stopOpacity="0" />
          <stop offset="20%" stopColor="var(--accent-primary)" stopOpacity="0.6" />
          <stop offset="80%" stopColor="var(--accent-glow)" stopOpacity="0.8" />
          <stop offset="100%" stopColor="var(--accent-glow)" stopOpacity="0" />
        </linearGradient>
      </defs>
      {/* Wavy flight path */}
      <path
        d="M0 6 Q25 2, 50 6 T100 6 T150 6 T200 6"
        fill="none"
        stroke="url(#flightPathGradient)"
        strokeWidth="2"
        strokeLinecap="round"
        strokeDasharray="200"
        strokeDashoffset={isVisible ? '0' : '200'}
        className="transition-all duration-1000 ease-out"
      />
      {/* Dotted waypoints */}
      {[40, 80, 120, 160].map((x) => (
        <circle
          key={x}
          cx={x}
          cy="6"
          r="1.5"
          fill="var(--accent-glow)"
          className={cn(
            'transition-all duration-500',
            isVisible ? 'opacity-60' : 'opacity-0'
          )}
          style={{ transitionDelay: `${x * 3}ms` }}
        />
      ))}
    </svg>
  )
}

export function Header({ isRunning, port = 8318, version = 'v0.1.0' }: HeaderProps) {
  const [isLogoHovered, setIsLogoHovered] = useState(false)
  const [isSettingsOpen, setIsSettingsOpen] = useState(false)

  return (
    <>
      <SettingsPanel isOpen={isSettingsOpen} onClose={() => setIsSettingsOpen(false)} />
    <header
      className={cn(
        'relative h-16 flex items-center justify-between px-6',
        'bg-[var(--bg-panel)] border-b border-[var(--border-subtle)]',
        // Instrument panel gradient overlay
        'before:absolute before:inset-0 before:pointer-events-none',
        'before:bg-gradient-to-b before:from-white/[0.02] before:to-transparent'
      )}
    >
      {/* Left: Logo + Title + Version */}
      <div
        className="relative flex items-center gap-4"
        onMouseEnter={() => setIsLogoHovered(true)}
        onMouseLeave={() => setIsLogoHovered(false)}
      >
        {/* Paper Airplane Logo */}
        <div
          className={cn(
            'relative w-11 h-11 flex items-center justify-center',
            'rounded-lg',
            'bg-gradient-to-br from-[var(--accent-primary)] to-[var(--accent-secondary)]',
            'shadow-[0_2px_0_0_oklch(0_0_0/0.2)]',
            'transition-all duration-300',
            isLogoHovered && 'shadow-[0_0_20px_0_var(--accent-glow)]',
            isRunning && 'animate-glow-pulse'
          )}
        >
          <PaperAirplaneLogo
            className="text-[var(--bg-panel)]"
            isHovered={isLogoHovered}
          />
        </div>

        {/* Title + Flight Path */}
        <div className="relative flex flex-col">
          <div className="flex items-center gap-3">
            {/* PROXYPILOT title */}
            <span
              className={cn(
                'text-xl font-bold uppercase tracking-[0.2em]',
                'text-[var(--text-primary)]',
                'transition-colors duration-300'
              )}
              style={{ fontFamily: 'var(--font-display)' }}
            >
              PROXYPILOT
            </span>

            {/* Version Badge */}
            <span
              className={cn(
                'px-2 py-0.5 rounded text-[10px] font-medium uppercase tracking-wider',
                'bg-[var(--bg-elevated)] border border-[var(--border-subtle)]',
                'text-[var(--text-muted)]'
              )}
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              {version}
            </span>
          </div>

          {/* Flight Path Animation */}
          <FlightPathLine isVisible={isLogoHovered} />
        </div>
      </div>

      {/* Center Divider - Instrument Panel Style */}
      <div
        className={cn(
          'hidden md:block h-8 w-px mx-4',
          'bg-gradient-to-b from-transparent via-[var(--border-default)] to-transparent'
        )}
      />

      {/* Right: Status + Actions */}
      <div className="flex items-center gap-4">
        {/* Status Indicator - Cockpit Style */}
        <div
          className={cn(
            'flex items-center gap-2.5 px-4 py-2 rounded-lg',
            'font-medium text-sm tracking-wide',
            'border transition-all duration-300',
            isRunning
              ? [
                  'bg-[oklch(0.72_0.19_145/0.08)]',
                  'border-[oklch(0.72_0.19_145/0.4)]',
                  'text-[oklch(0.72_0.19_145)]',
                  'shadow-[0_0_16px_0_oklch(0.72_0.19_145/0.25),inset_0_1px_0_0_oklch(0.72_0.19_145/0.1)]',
                ].join(' ')
              : [
                  'bg-[var(--bg-elevated)]',
                  'border-[var(--border-subtle)]',
                  'text-[var(--text-muted)]',
                ].join(' ')
          )}
        >
          {/* Status Dot with Glow Ring */}
          <span className="relative flex items-center justify-center">
            {/* Outer glow ring (only when running) */}
            {isRunning && (
              <span
                className={cn(
                  'absolute w-4 h-4 rounded-full',
                  'bg-[oklch(0.72_0.19_145/0.3)]',
                  'animate-radar-pulse'
                )}
              />
            )}
            {/* Core dot */}
            <span
              className={cn(
                'relative w-2.5 h-2.5 rounded-full',
                isRunning
                  ? 'bg-[oklch(0.72_0.19_145)] shadow-[0_0_8px_0_oklch(0.72_0.19_145)]'
                  : 'bg-[var(--text-muted)]'
              )}
            />
          </span>

          {/* Status Text */}
          <span
            className="uppercase tracking-wider"
            style={{ fontFamily: 'var(--font-mono)' }}
          >
            {isRunning ? 'ONLINE' : 'OFFLINE'}
          </span>

          {/* Port indicator (only when running) */}
          {isRunning && (
            <span
              className={cn(
                'ml-1 px-1.5 py-0.5 rounded text-[10px]',
                'bg-[oklch(0.72_0.19_145/0.15)]',
                'text-[oklch(0.72_0.19_145/0.8)]'
              )}
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              :{port}
            </span>
          )}
        </div>

        {/* Instrument Panel Separator */}
        <div
          className={cn(
            'h-6 w-px',
            'bg-[var(--border-subtle)]'
          )}
        />

        {/* Action Buttons */}
        <div className="flex items-center gap-1">
          {/* Theme Toggle */}
          <ThemeToggle />

          {/* Settings Button */}
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setIsSettingsOpen(true)}
            className={cn(
              'text-[var(--text-muted)]',
              'hover:text-[var(--text-primary)]',
              'hover:bg-[var(--bg-active)]'
            )}
            title="Settings"
          >
            <Settings className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Bottom Accent Line - Cockpit Trim */}
      <div
        className={cn(
          'absolute bottom-0 left-0 right-0 h-px',
          'bg-gradient-to-r from-transparent via-[var(--accent-primary)] to-transparent',
          'opacity-20'
        )}
      />
    </header>
    </>
  )
}
