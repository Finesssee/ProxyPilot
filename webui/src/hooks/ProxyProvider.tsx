import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { EngineOfflineError, ProxyContext, type ProxyStatus } from './useProxyContext'

const SERVICE_PREFLIGHT_CACHE_MS = 5000

export function ProxyProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<ProxyStatus | null>(null)
  const statusRef = useRef<ProxyStatus | null>(null)
  const servicePreflightCacheRef = useRef<{ at: number; status: ProxyStatus } | null>(null)
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
      return
    }
    toast.error(message)
  }, [])

  const fetchServicePreflight = useCallback(async (force = false) => {
    const cached = servicePreflightCacheRef.current
    if (!force && cached && cached.status.running && Date.now() - cached.at < SERVICE_PREFLIGHT_CACHE_MS) {
      return cached.status
    }

    let nextStatus: ProxyStatus
    if (window.pp_status) {
      nextStatus = await window.pp_status()
    } else {
      try {
        const res = await fetch('/healthz')
        if (!res.ok) {
          nextStatus = {
            running: false,
            port: 0,
            base_url: location.origin,
            last_error: res.statusText,
          }
        } else {
          const body = await res.json().catch(() => ({}))
          nextStatus = {
            running: true,
            port: typeof body.port === 'number' ? body.port : 0,
            base_url: location.origin,
            last_error: '',
          }
        }
      } catch (fetchErr) {
        nextStatus = {
          running: false,
          port: 0,
          base_url: location.origin,
          last_error: String(fetchErr),
        }
      }
    }

    servicePreflightCacheRef.current = nextStatus.running ? { at: Date.now(), status: nextStatus } : null
    return nextStatus
  }, [])

  const refreshStatus = useCallback(async (force = false) => {
    try {
      const nextStatus = await fetchServicePreflight(force)
      setStatus(nextStatus)
      statusRef.current = nextStatus
      const isRunning = nextStatus.running

      if (isRunning) {
        setWasRunning(true)
        setUserStopped(false)
        setRetryDelay(1200)
        if (mgmtKey) {
          try {
            const headers = { 'X-Management-Key': mgmtKey }
            const res = await fetch('/v0/management/auth-files', { headers })
            if (res.ok) {
              const files = await res.json()
              if (Array.isArray(files)) {
                setAuthFiles(files.filter((file): file is string => typeof file === 'string'))
              }
            }
          } catch (error) {
            console.error('Auth files error:', error)
          }
        }
        return
      }

      if (wasRunning && !userStopped && isDesktop && window.pp_start) {
        console.log('Unexpected stop detected, attempting auto-reconnect...')
        window.pp_start().catch(error => console.error('Auto-reconnect failed:', error))
      }
      setAuthFiles([])
      setRetryDelay(prev => Math.min(prev * 1.5, 10000))
    } catch (error) {
      console.error('Status error:', error)
      setRetryDelay(prev => Math.min(prev * 1.5, 10000))
    }
  }, [fetchServicePreflight, isDesktop, mgmtKey, userStopped, wasRunning])

  const mgmtFetch = useCallback(async (path: string, opts: RequestInit = {}) => {
    const currentStatus = statusRef.current
    if (!currentStatus?.running) {
      throw new EngineOfflineError()
    }
    if (!mgmtKey) {
      throw new Error('Missing management key')
    }

    setIsMgmtLoading(true)
    try {
      const headers = Object.assign({}, opts.headers || {}, { 'X-Management-Key': mgmtKey })
      const res = await fetch(path, { ...opts, headers })
      const contentType = (res.headers.get('content-type') || '').toLowerCase()
      const body = contentType.includes('application/json') ? await res.json() : await res.text()
      if (!res.ok) {
        const latestStatus = statusRef.current
        if (res.status === 404 && (!latestStatus || !latestStatus.running)) {
          throw new EngineOfflineError()
        }
        const message =
          typeof body === 'string'
            ? body
            : typeof body?.error === 'string'
              ? body.error
              : JSON.stringify(body)
        throw new Error(`${res.status} ${res.statusText}: ${message}`)
      }
      return body
    } catch (error) {
      if (error instanceof TypeError) {
        throw new EngineOfflineError()
      }
      throw error
    } finally {
      setIsMgmtLoading(false)
    }
  }, [mgmtKey])

  const handleAction = useCallback(async (
    action: (() => Promise<void>) | undefined,
    actionId: string,
    successMsg: string,
  ) => {
    if (!action) {
      return
    }
    if (actionId === 'stop') {
      setUserStopped(true)
    }
    if (actionId === 'start' || actionId === 'restart') {
      setUserStopped(false)
    }
    setLoading(actionId)
    try {
      await action()
      showToast(successMsg, 'success')
      await refreshStatus(true)
    } catch (error) {
      showToast(error instanceof Error ? error.message : String(error), 'error')
    } finally {
      setLoading(null)
    }
  }, [refreshStatus, showToast])

  useEffect(() => {
    refreshStatus()

    let timeoutId: ReturnType<typeof setTimeout> | undefined
    const scheduleRefresh = () => {
      timeoutId = setTimeout(async () => {
        await refreshStatus()
        scheduleRefresh()
      }, retryDelay)
    }
    scheduleRefresh()

    void (async () => {
      try {
        if (window.pp_get_management_key) {
          const key = await window.pp_get_management_key()
          setMgmtKey(key)
          return
        }
        if (!isDesktop) {
          const meta = document.querySelector('meta[name="pp-mgmt-key"]')
          setMgmtKey(meta ? meta.getAttribute('content') : null)
        }
      } catch (error) {
        console.error('Management key error:', error)
      }
    })()

    return () => {
      if (timeoutId) {
        clearTimeout(timeoutId)
      }
    }
  }, [isDesktop, refreshStatus, retryDelay])

  return (
    <ProxyContext.Provider
      value={{
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
      }}
    >
      {children}
    </ProxyContext.Provider>
  )
}
