import { createContext, useContext, useEffect, useState, useCallback } from 'react'
import type { ReactNode } from 'react'

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
  }
}

export interface ProxyStatus {
  running: boolean;
  port: number;
  base_url: string;
  last_error: string;
}

interface ProxyContextType {
  status: ProxyStatus | null;
  isDesktop: boolean;
  mgmtKey: string | null;
  loading: string | null;
  setLoading: (id: string | null) => void;
  refreshStatus: () => Promise<void>;
  showToast: (message: string, type?: 'success' | 'error') => void;
  toast: { message: string; type: 'success' | 'error' } | null;
  mgmtFetch: (path: string, opts?: RequestInit) => Promise<any>;
  handleAction: (action: (() => Promise<void>) | undefined, actionId: string, successMsg: string) => Promise<void>;
}

const ProxyContext = createContext<ProxyContextType | null>(null)

export function useProxyContext() {
  const ctx = useContext(ProxyContext)
  if (!ctx) throw new Error('useProxyContext must be used within ProxyProvider')
  return ctx
}

export function ProxyProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<ProxyStatus | null>(null)
  const [mgmtKey, setMgmtKey] = useState<string | null>(null)
  const [loading, setLoading] = useState<string | null>(null)
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null)

  const isDesktop = typeof window.pp_status === 'function'

  const showToast = useCallback((message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 2500)
  }, [])

  const refreshStatus = useCallback(async () => {
    try {
      if (window.pp_status) {
        const s = await window.pp_status()
        setStatus(s)
      } else {
        const res = await fetch('/healthz')
        if (!res.ok) {
          setStatus({ running: false, port: 0, base_url: location.origin, last_error: res.statusText })
          return
        }
        const body = await res.json().catch(() => ({}))
        setStatus({
          running: true,
          port: body.port || 0,
          base_url: location.origin,
          last_error: '',
        })
      }
    } catch (e) {
      console.error('Status error:', e)
    }
  }, [])

  const mgmtFetch = useCallback(async (path: string, opts: RequestInit = {}) => {
    if (!mgmtKey) throw new Error('Missing management key')
    const headers = Object.assign({}, opts.headers || {}, { 'X-Management-Key': mgmtKey })
    const res = await fetch(path, { ...opts, headers })
    const ct = (res.headers.get('content-type') || '').toLowerCase()
    const body = ct.includes('application/json') ? await res.json() : await res.text()
    if (!res.ok) {
      const msg = typeof body === 'string' ? body : body?.error ? body.error : JSON.stringify(body)
      throw new Error(`${res.status} ${res.statusText}: ${msg}`)
    }
    return body
  }, [mgmtKey])

  const handleAction = useCallback(async (
    action: (() => Promise<void>) | undefined,
    actionId: string,
    successMsg: string
  ) => {
    if (!action) return
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
    const interval = setInterval(refreshStatus, 1200)

    ;(async () => {
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

    return () => clearInterval(interval)
  }, [refreshStatus, isDesktop])

  return (
    <ProxyContext.Provider value={{
      status,
      isDesktop,
      mgmtKey,
      loading,
      setLoading,
      refreshStatus,
      showToast,
      toast,
      mgmtFetch,
      handleAction,
    }}>
      {children}
    </ProxyContext.Provider>
  )
}
