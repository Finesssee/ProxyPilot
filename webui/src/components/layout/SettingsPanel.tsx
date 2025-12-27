import { useState, useEffect } from 'react'
import { X, FolderOpen, Clipboard, Lock, LockOpen, RefreshCw, Download, CheckCircle2, AlertCircle, Loader2, Play, Shield, ExternalLink } from 'lucide-react'
import { Button } from '../ui/button'
import { Switch } from '../ui/switch'
import { Label } from '../ui/label'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'
import { useProxyContext } from '@/hooks/useProxyContext'

interface SettingsPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface UpdateInfo {
  available: boolean
  version: string
  download_url: string
  release_notes?: string
}

interface UpdateStatus {
  downloading: boolean
  progress: { total_bytes: number; downloaded_bytes: number; percent: number }
  ready: boolean
  error: string
}

type UpdateStage = 'idle' | 'checking' | 'available' | 'downloading' | 'verifying' | 'ready' | 'installing'

export function SettingsPanel({ isOpen, onClose }: SettingsPanelProps) {
  const { mgmtFetch } = useProxyContext()
  const [privateOAuth, setPrivateOAuth] = useState(false)
  const [checkingUpdates, setCheckingUpdates] = useState(false)
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [updateStage, setUpdateStage] = useState<UpdateStage>('idle')
  const [downloadProgress, setDownloadProgress] = useState(0)
  const [updateError, setUpdateError] = useState<string | null>(null)

  // Load settings on mount
  useEffect(() => {
    ;(async () => {
      try {
        if (window.pp_get_oauth_private) {
          const priv = await window.pp_get_oauth_private()
          setPrivateOAuth(priv)
        }
      } catch (e) {
        console.error('Failed to load OAuth private setting:', e)
      }
    })()
  }, [])

  // Poll for download status when downloading
  useEffect(() => {
    if (updateStage !== 'downloading') return

    const pollStatus = async () => {
      try {
        let status: UpdateStatus
        if (mgmtFetch) {
          status = await mgmtFetch('/v0/management/updates/status')
        } else {
          return
        }

        if (status.error) {
          setUpdateError(status.error)
          setUpdateStage('available')
          return
        }

        if (status.downloading) {
          setDownloadProgress(status.progress?.percent || 0)
        } else if (status.ready) {
          setUpdateStage('ready')
          setDownloadProgress(100)
        }
      } catch (e) {
        console.error('Failed to poll update status:', e)
      }
    }

    const interval = setInterval(pollStatus, 500)
    return () => clearInterval(interval)
  }, [updateStage, mgmtFetch])

  const handlePrivateOAuthChange = async (checked: boolean) => {
    try {
      if (window.pp_set_oauth_private) {
        await window.pp_set_oauth_private(checked)
        setPrivateOAuth(checked)
        toast.success(checked ? 'Private browsing enabled' : 'Private browsing disabled')
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleOpenLogs = async () => {
    try {
      if (window.pp_open_logs) {
        await window.pp_open_logs()
        toast.success('Opened logs folder')
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleOpenAuthFolder = async () => {
    try {
      if (window.pp_open_auth_folder) {
        await window.pp_open_auth_folder()
        toast.success('Opened auth folder')
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleCopyDiagnostics = async () => {
    try {
      if (window.pp_copy_diagnostics) {
        await window.pp_copy_diagnostics()
        toast.success('Diagnostics copied to clipboard')
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleCheckUpdates = async () => {
    setCheckingUpdates(true)
    setUpdateStage('checking')
    setUpdateError(null)
    try {
      let info: UpdateInfo | null = null
      if (window.pp_check_updates) {
        info = await window.pp_check_updates()
      } else if (mgmtFetch) {
        info = await mgmtFetch('/v0/management/updates/check')
      }

      setUpdateInfo(info)
      if (info?.available) {
        setUpdateStage('available')
        toast.success(`Update available: v${info.version}`)
      } else {
        setUpdateStage('idle')
        toast.success('You are on the latest version')
      }
    } catch (e) {
      setUpdateStage('idle')
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setCheckingUpdates(false)
    }
  }

  const handleDownloadUpdate = async () => {
    if (!updateInfo?.version) return
    setUpdateStage('downloading')
    setDownloadProgress(0)
    setUpdateError(null)

    try {
      if (mgmtFetch) {
        await mgmtFetch('/v0/management/updates/download', {
          method: 'POST',
          body: JSON.stringify({ version: updateInfo.version }),
        })
      } else if (window.pp_download_update) {
        // Fallback to opening download page
        await window.pp_download_update(updateInfo.download_url)
        setUpdateStage('available')
        toast.success('Opening download page...')
      }
    } catch (e) {
      setUpdateStage('available')
      setUpdateError(e instanceof Error ? e.message : String(e))
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleInstallUpdate = async () => {
    setUpdateStage('installing')
    try {
      if (mgmtFetch) {
        const result = await mgmtFetch('/v0/management/updates/install', {
          method: 'POST',
        })
        if (result?.success) {
          toast.success(result.message || 'Update installed! Restarting...')
          // Application will restart
        } else {
          throw new Error(result?.message || 'Installation failed')
        }
      }
    } catch (e) {
      setUpdateStage('ready')
      setUpdateError(e instanceof Error ? e.message : String(e))
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleOpenReleasePage = () => {
    if (updateInfo?.download_url) {
      window.open(updateInfo.download_url, '_blank')
    }
  }

  if (!isOpen) return null

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/50 backdrop-blur-sm z-40 animate-in fade-in duration-200"
        onClick={onClose}
      />

      {/* Panel */}
      <div
        className={cn(
          'fixed right-0 top-0 bottom-0 w-80 z-50',
          'bg-[var(--bg-panel)] border-l border-[var(--border-subtle)]',
          'shadow-2xl',
          'animate-in slide-in-from-right duration-300'
        )}
      >
        {/* Header */}
        <div
          className={cn(
            'flex items-center justify-between px-5 py-4',
            'border-b border-[var(--border-subtle)]'
          )}
        >
          <h2
            className="text-sm font-bold uppercase tracking-[0.15em] text-[var(--text-primary)]"
            style={{ fontFamily: 'var(--font-display)' }}
          >
            Settings
          </h2>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onClose}
            className="text-[var(--text-muted)] hover:text-[var(--text-primary)]"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Content */}
        <div className="p-5 space-y-6">
          {/* OAuth Settings */}
          <div className="space-y-4">
            <h3
              className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]"
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              Authentication
            </h3>

            <div
              className={cn(
                'flex items-center justify-between p-3 rounded-lg',
                'bg-[var(--bg-elevated)] border border-[var(--border-subtle)]'
              )}
            >
              <div className="flex items-center gap-3">
                {privateOAuth ? (
                  <Lock className="h-4 w-4 text-[var(--accent-glow)]" />
                ) : (
                  <LockOpen className="h-4 w-4 text-[var(--text-muted)]" />
                )}
                <div>
                  <Label
                    htmlFor="private-oauth"
                    className="text-sm text-[var(--text-primary)] cursor-pointer"
                  >
                    Private Browsing
                  </Label>
                  <p className="text-xs text-[var(--text-muted)]">
                    Use InPrivate mode for OAuth
                  </p>
                </div>
              </div>
              <Switch
                id="private-oauth"
                checked={privateOAuth}
                onCheckedChange={handlePrivateOAuthChange}
              />
            </div>
          </div>

          {/* Folders */}
          <div className="space-y-4">
            <h3
              className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]"
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              Folders
            </h3>

            <div className="space-y-2">
              <Button
                variant="outline"
                className="w-full justify-start gap-3"
                onClick={handleOpenLogs}
              >
                <FolderOpen className="h-4 w-4" />
                Open Logs Folder
              </Button>

              <Button
                variant="outline"
                className="w-full justify-start gap-3"
                onClick={handleOpenAuthFolder}
              >
                <FolderOpen className="h-4 w-4" />
                Open Auth Folder
              </Button>
            </div>
          </div>

          {/* Diagnostics */}
          <div className="space-y-4">
            <h3
              className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]"
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              Diagnostics
            </h3>

            <Button
              variant="outline"
              className="w-full justify-start gap-3"
              onClick={handleCopyDiagnostics}
            >
              <Clipboard className="h-4 w-4" />
              Copy Diagnostics
            </Button>
          </div>

          {/* Updates */}
          <div className="space-y-4">
            <h3
              className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]"
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              Updates
            </h3>

            {/* Check for Updates Button */}
            <Button
              variant="outline"
              className="w-full justify-start gap-3"
              onClick={handleCheckUpdates}
              disabled={checkingUpdates || updateStage === 'downloading' || updateStage === 'installing'}
            >
              {checkingUpdates ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Check for Updates
            </Button>

            {/* Update Status Card */}
            {(updateInfo || updateStage !== 'idle') && (
              <div
                className={cn(
                  'p-4 rounded-lg border text-sm space-y-3',
                  updateStage === 'ready' || updateStage === 'installing'
                    ? 'bg-emerald-500/10 border-emerald-500/30'
                    : updateInfo?.available
                    ? 'bg-[var(--accent-primary)]/10 border-[var(--accent-primary)]/30'
                    : 'bg-green-500/10 border-green-500/30'
                )}
              >
                {/* Header */}
                <div className="flex items-start gap-2">
                  {updateStage === 'ready' ? (
                    <Shield className="h-4 w-4 text-emerald-500 mt-0.5 shrink-0" />
                  ) : updateStage === 'installing' ? (
                    <Loader2 className="h-4 w-4 text-emerald-500 mt-0.5 shrink-0 animate-spin" />
                  ) : updateInfo?.available ? (
                    <AlertCircle className="h-4 w-4 text-[var(--accent-primary)] mt-0.5 shrink-0" />
                  ) : (
                    <CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 shrink-0" />
                  )}
                  <div className="flex-1 min-w-0">
                    <p className="font-medium text-[var(--text-primary)]">
                      {updateStage === 'downloading' && 'Downloading...'}
                      {updateStage === 'ready' && `v${updateInfo?.version} ready to install`}
                      {updateStage === 'installing' && 'Installing update...'}
                      {updateStage === 'available' && `v${updateInfo?.version} available`}
                      {updateStage === 'idle' && !updateInfo?.available && 'Up to date'}
                    </p>
                    {updateInfo?.release_notes && !updateInfo?.available && (
                      <p className="text-xs text-[var(--text-muted)] mt-1">
                        {updateInfo.release_notes}
                      </p>
                    )}
                  </div>
                </div>

                {/* Download Progress */}
                {updateStage === 'downloading' && (
                  <div className="space-y-2">
                    <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--bg-void)]">
                      <div
                        className="h-full bg-[var(--accent-primary)] transition-all duration-300"
                        style={{ width: `${downloadProgress}%` }}
                      />
                    </div>
                    <p className="text-xs text-[var(--text-muted)] text-center">
                      {downloadProgress.toFixed(0)}% complete
                    </p>
                  </div>
                )}

                {/* Error Message */}
                {updateError && (
                  <div className="p-2 rounded bg-red-500/10 border border-red-500/30 text-red-400 text-xs">
                    {updateError}
                  </div>
                )}

                {/* Action Buttons */}
                <div className="flex gap-2">
                  {updateStage === 'available' && (
                    <>
                      <Button
                        size="sm"
                        onClick={handleDownloadUpdate}
                        className="flex-1 gap-2"
                      >
                        <Download className="h-3 w-3" />
                        Download & Install
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={handleOpenReleasePage}
                        className="gap-2"
                      >
                        <ExternalLink className="h-3 w-3" />
                      </Button>
                    </>
                  )}

                  {updateStage === 'ready' && (
                    <Button
                      size="sm"
                      onClick={handleInstallUpdate}
                      className="flex-1 gap-2 bg-emerald-600 hover:bg-emerald-500"
                    >
                      <Play className="h-3 w-3" />
                      Install & Restart
                    </Button>
                  )}

                  {updateStage === 'installing' && (
                    <div className="flex-1 text-center text-xs text-emerald-400">
                      Please wait, do not close the application...
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Version */}
          <div className="pt-4 border-t border-[var(--border-subtle)]">
            <p
              className="text-xs text-center text-[var(--text-muted)]"
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              ProxyPilot v0.1.0
            </p>
          </div>
        </div>
      </div>
    </>
  )
}
