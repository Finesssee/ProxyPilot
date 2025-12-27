import { useState, useEffect } from 'react'
import { X, FolderOpen, Clipboard, Lock, LockOpen, RefreshCw, Download, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react'
import { Button } from '../ui/button'
import { Switch } from '../ui/switch'
import { Label } from '../ui/label'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'

interface SettingsPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface UpdateInfo {
  available: boolean
  version: string
  download_url: string
}

export function SettingsPanel({ isOpen, onClose }: SettingsPanelProps) {
  const [privateOAuth, setPrivateOAuth] = useState(false)
  const [checkingUpdates, setCheckingUpdates] = useState(false)
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)

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
    try {
      if (window.pp_check_updates) {
        const info = await window.pp_check_updates()
        setUpdateInfo(info)
        if (info?.available) {
          toast.success(`Update available: ${info.version}`)
        } else {
          toast.success('You are on the latest version')
        }
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setCheckingUpdates(false)
    }
  }

  const handleDownloadUpdate = async () => {
    if (!updateInfo?.download_url) return
    try {
      if (window.pp_download_update) {
        await window.pp_download_update(updateInfo.download_url)
        toast.success('Opening download page...')
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
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

            <Button
              variant="outline"
              className="w-full justify-start gap-3"
              onClick={handleCheckUpdates}
              disabled={checkingUpdates}
            >
              {checkingUpdates ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Check for Updates
            </Button>

            {updateInfo && (
              <div
                className={cn(
                  'p-3 rounded-lg border text-sm',
                  updateInfo.available
                    ? 'bg-[var(--accent-primary)]/10 border-[var(--accent-primary)]/30'
                    : 'bg-green-500/10 border-green-500/30'
                )}
              >
                <div className="flex items-start gap-2">
                  {updateInfo.available ? (
                    <AlertCircle className="h-4 w-4 text-[var(--accent-primary)] mt-0.5 shrink-0" />
                  ) : (
                    <CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 shrink-0" />
                  )}
                  <div className="flex-1 min-w-0">
                    <p className="font-medium text-[var(--text-primary)]">
                      {updateInfo.available ? `v${updateInfo.version} available` : 'Up to date'}
                    </p>
                    {updateInfo.available && (
                      <Button
                        size="sm"
                        onClick={handleDownloadUpdate}
                        className="mt-2 gap-2"
                      >
                        <Download className="h-3 w-3" />
                        Download
                      </Button>
                    )}
                  </div>
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
