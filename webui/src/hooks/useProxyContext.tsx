import { createContext, useContext, useEffect, useState, useCallback, useRef } from 'react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'

// Type declarations for desktop app bindings
declare global {
  interface Window {
    pp_status?: () => Promise<ProxyStatus>;
    pp_start?: () => Promise<void>;
    pp_stop?: () => Promise<void>;
    pp_restart?: () => Promise<void>;
    pp_open_logs?: () => Promise<void>;
    pp_open_auth_folder?: () => Promise<void>;
    pp_open_legacy_ui?: () => Promise<void>;
    pp_open_diagnostics?: () => Promise<void>;
    pp_get_oauth_private?: () => Promise<boolean>;
    pp_set_oauth_private?: (enabled: boolean) => Promise<void>;
    pp_oauth?: (provider: string) => Promise<void>;
    pp_copy_diagnostics?: () => Promise<void>;
    pp_get_management_key?: () => Promise<string>;
    pp_get_requests?: () => Promise<any>;
    pp_get_usage?: () => Promise<any>;
    pp_detect_agents?: () => Promise<any[]>;
    pp_configure_agent?: (agentId: string) => Promise<void>;
    pp_unconfigure_agent?: (agentId: string) => Promise<void>;
    pp_check_updates?: () => Promise<any>;
    pp_download_update?: (url: string) => Promise<void>;
  }
}

export interface ProxyStatus {
  running: boolean;
  version?: string;
  port: number;
  base_url: string;
  last_error: string;
}

export class EngineOfflineError extends Error {
  constructor() {
    super('Engine Offline')
    this.name = 'EngineOfflineError'
  }
}

interface ProxyContextType {
  status: ProxyStatus | null;
  isDesktop: boolean;
  mgmtKey: string | null;
  authFiles: string[];
  loading: string | null;
  isMgmtLoading: boolean;
  setLoading: (id: string | null) => void;
  refreshStatus: () => Promise<void>;
  showToast: (message: string, type?: 'success' | 'error') => void;
  mgmtFetch: (path: string, opts?: RequestInit) => Promise<any>;
  handleAction: (action: (() => Promise<void>) | undefined, actionId: string, successMsg: string) => Promise<void>;
  pp_get_usage?: () => Promise<any>;
  pp_detect_agents?: () => Promise<any[]>;
  pp_configure_agent?: (agentId: string) => Promise<void>;
  pp_unconfigure_agent?: (agentId: string) => Promise<void>;
  pp_check_updates?: () => Promise<any>;
  pp_download_update?: (url: string) => Promise<void>;
}

const ProxyContext = createContext<ProxyContextType | null>(null)

export function useProxyContext() {
  const ctx = useContext(ProxyContext)
  if (!ctx) throw new Error('useProxyContext must be used within ProxyProvider')
  return ctx
}

export function ProxyProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<ProxyStatus | null>(null)
  const statusRef = useRef<ProxyStatus | null>(null)
  const [mgmtKey, setMgmtKey] = useState<string | null>(null)
  const [authFiles, setAuthFiles] = useState<string[]>([])
  const [loading, setLoading] = useState<string | null>(null)
  const [isMgmtLoading, setIsMgmtLoading] = useState(false)
  const [retryDelay, setRetryDelay] = useState(1200)
  const [wasRunning, setWasRunning] = useState(false)
  const [userStopped, setUserStopped] = useState(false)

  const isDesktop = typeof window.pp_status === 'function'

  const showToast = useCallback((message: string, type: 'success' | 'error' = 'success') => {
    if (type === 'success') {
      toast.success(message)
    } else {
      toast.error(message)
    }
  }, [])

  const refreshStatus = useCallback(async () => {
    try {
      let isRunning = false
      if (window.pp_status) {
        const s = await window.pp_status()
        setStatus(s)
        statusRef.current = s
        isRunning = s.running
      } else {
        try {
          const res = await fetch('/healthz')
          if (!res.ok) {
            const s = { running: false, port: 0, base_url: location.origin, last_error: res.statusText }
            setStatus(s)
            statusRef.current = s
          } else {
            const body = await res.json().catch(() => ({}))
            const s = {
              running: true,
              port: body.port || 0,
              base_url: location.origin,
              last_error: '',
            }
            setStatus(s)
            statusRef.current = s
            isRunning = true
          }
        } catch (fetchErr) {
          const s = { running: false, port: 0, base_url: location.origin, last_error: String(fetchErr) }
          setStatus(s)
          statusRef.current = s
          isRunning = false
        }
      }

      if (isRunning) {
        setWasRunning(true)
        setUserStopped(false)
        setRetryDelay(1200) // Reset delay on success
        if (mgmtKey) {
          try {
            const headers = { 'X-Management-Key': mgmtKey }
            const res = await fetch('/v0/management/auth-files', { headers })
            if (res.ok) {
              const files = await res.json()
              if (Array.isArray(files)) {
                setAuthFiles(files)
              }
            }
          } catch (e) {
            console.error('Auth files error:', e)
          }
        }
      } else {
        if (wasRunning && !userStopped && isDesktop && window.pp_start) {
          console.log('Unexpected stop detected, attempting auto-reconnect...')
          window.pp_start().catch(e => console.error('Auto-reconnect failed:', e))
        }
        setAuthFiles([])
        setRetryDelay(prev => Math.min(prev * 1.5, 10000)) // Increase delay on failure
      }
    } catch (e) {
      console.error('Status error:', e)
      setRetryDelay(prev => Math.min(prev * 1.5, 10000))
    }
  }, [mgmtKey])

  const mgmtFetch = useCallback(async (path: string, opts: RequestInit = {}) => {
    const currentStatus = statusRef.current
    if (!currentStatus?.running) {
      throw new EngineOfflineError()
    }
    if (!mgmtKey) throw new Error('Missing management key')

    setIsMgmtLoading(true)
    try {
      const headers = Object.assign({}, opts.headers || {}, { 'X-Management-Key': mgmtKey })
      const res = await fetch(path, { ...opts, headers })
      const ct = (res.headers.get('content-type') || '').toLowerCase()
      const body = ct.includes('application/json') ? await res.json() : await res.text()
      if (!res.ok) {
        const latestStatus = statusRef.current
        if (res.status === 404 && (!latestStatus || !latestStatus.running)) {
          throw new EngineOfflineError()
        }
        const msg = typeof body === 'string' ? body : body?.error ? body.error : JSON.stringify(body)
        throw new Error(`${res.status} ${res.statusText}: ${msg}`)
      }
      return body
    } catch (e) {
      if (e instanceof TypeError) {
        // Network error
        throw new EngineOfflineError()
      }
      throw e
    } finally {
      setIsMgmtLoading(false)
    }
  }, [mgmtKey])

  const handleAction = useCallback(async (
    action: (() => Promise<void>) | undefined,
    actionId: string,
    successMsg: string
  ) => {
    if (!action) return
    if (actionId === 'stop') setUserStopped(true)
    if (actionId === 'start' || actionId === 'restart') setUserStopped(false)
    setLoading(actionId)
    try {
      await action()
      showToast(successMsg, 'success')
      await refreshStatus()
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(e), 'error')
    } finally {
      setLoading(null)
    }
  }, [showToast, refreshStatus])

  // Initialize on mount
  useEffect(() => {
    refreshStatus()

    let timeoutId: any
    const scheduleRefresh = () => {
      timeoutId = setTimeout(async () => {
        await refreshStatus()
        scheduleRefresh()
      }, retryDelay)
    }
    scheduleRefresh()

      ; (async () => {
        try {
          if (window.pp_get_management_key) {
            const key = await window.pp_get_management_key()
            setMgmtKey(key)
          } else if (!isDesktop) {
            const meta = document.querySelector('meta[name="pp-mgmt-key"]')
            setMgmtKey(meta ? meta.getAttribute('content') : null)
          }
        } catch (e) {
          console.error('Management key error:', e)
        }
      })()

    return () => clearTimeout(timeoutId)
  }, [refreshStatus, isDesktop, retryDelay])

  return (
    <ProxyContext.Provider value={{
      status,
      isDesktop,
      mgmtKey,
      authFiles,
      loading,
      isMgmtLoading,
      setLoading,
      refreshStatus,
      showToast,
      mgmtFetch,
      handleAction,
      pp_get_usage: window.pp_get_usage,
      pp_detect_agents: window.pp_detect_agents,
      pp_configure_agent: window.pp_configure_agent,
      pp_unconfigure_agent: window.pp_unconfigure_agent,
      pp_check_updates: window.pp_check_updates,
      pp_download_update: window.pp_download_update,
    }}>
      {children}
    </ProxyContext.Provider>
  )
}
