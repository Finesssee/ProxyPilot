import { useState, useEffect, useCallback } from 'react'
import { Download, X, Sparkles, ExternalLink, Loader2 } from 'lucide-react'
import { Button } from '../ui/button'
import { cn } from '@/lib/utils'
import { useProxyContext } from '@/hooks/useProxyContext'

interface UpdateInfo {
  available: boolean
  version: string
  download_url: string
  release_notes?: string
}

type BannerState = 'hidden' | 'checking' | 'available' | 'downloading' | 'dismissed'

/**
 * UpdateBanner - Proactive update notification that appears at the top of the dashboard
 *
 * Automatically checks for updates on mount and displays a dismissible banner
 * when a new version is available. Persists dismissal in sessionStorage so the
 * banner doesn't reappear until next session.
 */
export function UpdateBanner() {
  const { pp_check_updates, mgmtFetch, isDesktop } = useProxyContext()
  const [bannerState, setBannerState] = useState<BannerState>('hidden')
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [downloadProgress, setDownloadProgress] = useState(0)

  // Check for updates on mount
  const checkForUpdates = useCallback(async () => {
    // Skip if already dismissed this session
    const dismissed = sessionStorage.getItem('pp-update-dismissed')
    if (dismissed) {
      setBannerState('dismissed')
      return
    }

    setBannerState('checking')

    try {
      let info: UpdateInfo | null = null

      if (pp_check_updates) {
        // Desktop app - use WebView2 binding
        info = await pp_check_updates()
      } else if (mgmtFetch) {
        // Web mode - use management API
        try {
          info = await mgmtFetch('/v0/management/updates/check')
        } catch {
          // Silently fail - updates may not be available in web mode
        }
      }

      if (info?.available) {
        setUpdateInfo(info)
        setBannerState('available')
      } else {
        setBannerState('hidden')
      }
    } catch (e) {
      console.error('Update check failed:', e)
      setBannerState('hidden')
    }
  }, [pp_check_updates, mgmtFetch])

  useEffect(() => {
    // Delay check to let app fully initialize
    const timer = setTimeout(checkForUpdates, 2000)
    return () => clearTimeout(timer)
  }, [checkForUpdates])

  // Poll for download status when downloading
  useEffect(() => {
    if (bannerState !== 'downloading' || !mgmtFetch) return

    const pollStatus = async () => {
      try {
        const status = await mgmtFetch('/v0/management/updates/status')

        if (status.error) {
          setBannerState('available')
          return
        }

        if (status.downloading) {
          setDownloadProgress(status.progress?.percent || 0)
        } else if (status.ready) {
          // Update downloaded - hide banner (user can install from settings)
          setBannerState('hidden')
        }
      } catch {
        // Ignore polling errors
      }
    }

    const interval = setInterval(pollStatus, 500)
    return () => clearInterval(interval)
  }, [bannerState, mgmtFetch])

  const handleDownload = async () => {
    if (!updateInfo) return

    setBannerState('downloading')
    setDownloadProgress(0)

    try {
      if (mgmtFetch) {
        await mgmtFetch('/v0/management/updates/download', {
          method: 'POST',
          body: JSON.stringify({ version: updateInfo.version }),
        })
      } else if (updateInfo.download_url) {
        // Fallback - open release page
        window.open(updateInfo.download_url, '_blank')
        setBannerState('dismissed')
      }
    } catch (e) {
      console.error('Download failed:', e)
      setBannerState('available')
    }
  }

  const handleDismiss = () => {
    sessionStorage.setItem('pp-update-dismissed', updateInfo?.version || 'true')
    setBannerState('dismissed')
  }

  const handleOpenRelease = () => {
    if (updateInfo?.download_url) {
      window.open(updateInfo.download_url, '_blank')
    }
  }

  // Don't render if hidden or dismissed
  if (bannerState === 'hidden' || bannerState === 'dismissed' || bannerState === 'checking') {
    return null
  }

  return (
    <div
      className={cn(
        'relative flex items-center justify-between gap-4 px-4 py-2.5',
        'bg-gradient-to-r from-[var(--accent-primary)]/15 via-[var(--accent-glow)]/10 to-[var(--accent-primary)]/15',
        'border-b border-[var(--accent-primary)]/30',
        'animate-in slide-in-from-top duration-300'
      )}
    >
      {/* Decorative gradient line at top */}
      <div
        className={cn(
          'absolute top-0 left-0 right-0 h-px',
          'bg-gradient-to-r from-transparent via-[var(--accent-glow)] to-transparent',
          'opacity-60'
        )}
      />

      {/* Left: Update info */}
      <div className="flex items-center gap-3 min-w-0">
        <div
          className={cn(
            'flex items-center justify-center w-8 h-8 rounded-lg shrink-0',
            'bg-[var(--accent-primary)]/20 border border-[var(--accent-primary)]/40'
          )}
        >
          <Sparkles className="w-4 h-4 text-[var(--accent-glow)]" />
        </div>

        <div className="min-w-0">
          <p className="text-sm font-medium text-[var(--text-primary)] truncate">
            {bannerState === 'downloading' ? (
              <>Downloading v{updateInfo?.version}... {downloadProgress.toFixed(0)}%</>
            ) : (
              <>ProxyPilot v{updateInfo?.version} is available!</>
            )}
          </p>
          {updateInfo?.release_notes && bannerState === 'available' && (
            <p className="text-xs text-[var(--text-muted)] truncate">
              {updateInfo.release_notes}
            </p>
          )}
        </div>
      </div>

      {/* Right: Actions */}
      <div className="flex items-center gap-2 shrink-0">
        {bannerState === 'downloading' ? (
          <div className="flex items-center gap-2 text-[var(--text-muted)]">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span className="text-xs">Downloading...</span>
          </div>
        ) : (
          <>
            {/* Download button - only in desktop mode with mgmt API */}
            {isDesktop && mgmtFetch && (
              <Button
                size="sm"
                onClick={handleDownload}
                className={cn(
                  'gap-1.5 h-7 px-3 text-xs font-medium',
                  'bg-[var(--accent-primary)] hover:bg-[var(--accent-glow)]',
                  'text-white shadow-lg shadow-[var(--accent-primary)]/25'
                )}
              >
                <Download className="w-3 h-3" />
                Update Now
              </Button>
            )}

            {/* Release page link (always available) */}
            <Button
              size="sm"
              variant="ghost"
              onClick={handleOpenRelease}
              className="gap-1.5 h-7 px-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)]"
              title="View release notes"
            >
              <ExternalLink className="w-3 h-3" />
            </Button>

            {/* Dismiss button */}
            <Button
              size="sm"
              variant="ghost"
              onClick={handleDismiss}
              className="h-7 w-7 p-0 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
              title="Dismiss"
            >
              <X className="w-3.5 h-3.5" />
            </Button>
          </>
        )}
      </div>

      {/* Download progress bar */}
      {bannerState === 'downloading' && (
        <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-[var(--bg-void)]">
          <div
            className="h-full bg-[var(--accent-glow)] transition-all duration-300"
            style={{ width: `${downloadProgress}%` }}
          />
        </div>
      )}
    </div>
  )
}
