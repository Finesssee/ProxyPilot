import { useEffect, useState } from 'react'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Toaster } from 'sonner'
import { ProxyProvider, useProxyContext } from '@/hooks/useProxyContext'
import {
  EngineControl,
  ProviderLogins,
  Integrations,
  ModelMappings,
  MemoryManager,
  SemanticMemory,
  LogsViewer,
  ConfigEditor,
  RequestMonitor,
  UsageStats,
  AccountUsage,
  ThinkingBudgetSettings,
  CacheStats,
} from '@/components/dashboard'
import { Header } from '@/components/layout/Header'
import { StatusBar } from '@/components/layout/StatusBar'
import { IconRail, navigationItems } from '@/components/ui/icon-rail'

type ViewId = 'command' | 'providers' | 'routing' | 'memory' | 'logs' | 'requests' | 'analytics'

function DashboardContent() {
  const { status, isDesktop, mgmtKey } = useProxyContext()
  const [activeView, setActiveView] = useState<ViewId>('command')

  const isRunning = status?.running ?? false

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key >= '1' && e.key <= '7') {
        e.preventDefault()
        const index = parseInt(e.key, 10) - 1
        const item = navigationItems[index]
        if (item) {
          setActiveView(item.id as ViewId)
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  // Render view content based on activeView
  const renderViewContent = () => {
    switch (activeView) {
      case 'command':
        return (
          <div className="space-y-6">
            <div className="grid gap-6 lg:grid-cols-2">
              <EngineControl />
            </div>
            <Integrations />
          </div>
        )

      case 'providers':
        return (
          <div className="space-y-6">
            <ProviderLogins />

            {!isDesktop && !mgmtKey && (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key missing. Start ProxyPilot from the tray app to inject a local key,
                or set `MANAGEMENT_PASSWORD` before loading this page.
              </div>
            )}
          </div>
        )

      case 'routing':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <>
                <ModelMappings />
                <ThinkingBudgetSettings />
                <ConfigEditor />
              </>
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access routing settings.
              </div>
            )}
          </div>
        )

      case 'memory':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <>
                <MemoryManager />
                <SemanticMemory />
              </>
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access memory settings.
              </div>
            )}
          </div>
        )

      case 'logs':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <LogsViewer />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access logs.
              </div>
            )}
          </div>
        )

      case 'requests':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <RequestMonitor />
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access request monitor.
              </div>
            )}
          </div>
        )

      case 'analytics':
        return (
          <div className="space-y-6">
            {mgmtKey ? (
              <>
                <CacheStats />
                <UsageStats />
                <AccountUsage />
              </>
            ) : (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-300">
                Management key required to access analytics.
              </div>
            )}
          </div>
        )

      default:
        return null
    }
  }

  return (
    <div
      className="text-foreground transition-colors duration-500"
      style={{
        display: 'grid',
        gridTemplateRows: '64px 1fr 32px',
        gridTemplateColumns: '64px 1fr',
        height: '100vh',
        background: 'var(--bg-void)',
      }}
    >
      {/* Header - spans full width */}
      <div style={{ gridColumn: '1 / -1' }}>
        <Header isRunning={isRunning} port={status?.port} />
      </div>

      {/* Icon Rail - left side */}
      <div style={{ gridRow: 2 }}>
        <IconRail
          items={navigationItems}
          activeId={activeView}
          onSelect={(id) => setActiveView(id as ViewId)}
        />
      </div>

      {/* Main Content - fills remaining space */}
      <main
        key={activeView}
        className="animate-fade-in-up"
        style={{
          gridRow: 2,
          overflow: 'auto',
          padding: '24px',
        }}
      >
        {renderViewContent()}
      </main>

      {/* Status Bar - spans full width */}
      <div style={{ gridColumn: '1 / -1' }}>
        <StatusBar />
      </div>

      <Toaster position="bottom-center" theme="dark" />
    </div>
  )
}

export default function App() {
  return (
    <TooltipProvider>
      <ProxyProvider>
        <DashboardContent />
      </ProxyProvider>
    </TooltipProvider>
  )
}
