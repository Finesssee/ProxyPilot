import { createContext, useContext } from 'react'

type JsonPayload = Awaited<ReturnType<Response['json']>>

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
    pp_get_requests?: () => Promise<JsonPayload>;
    pp_get_usage?: () => Promise<JsonPayload>;
    pp_detect_agents?: () => Promise<Array<JsonPayload>>;
    pp_configure_agent?: (agentId: string) => Promise<void>;
    pp_unconfigure_agent?: (agentId: string) => Promise<void>;
    pp_check_updates?: () => Promise<JsonPayload>;
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

export interface ProxyContextType {
  status: ProxyStatus | null;
  isDesktop: boolean;
  mgmtKey: string | null;
  authFiles: string[];
  loading: string | null;
  isMgmtLoading: boolean;
  setLoading: (id: string | null) => void;
  refreshStatus: () => Promise<void>;
  showToast: (message: string, type?: 'success' | 'error') => void;
  mgmtFetch: (path: string, opts?: RequestInit) => Promise<JsonPayload | string>;
  handleAction: (action: (() => Promise<void>) | undefined, actionId: string, successMsg: string) => Promise<void>;
  pp_get_usage?: () => Promise<JsonPayload>;
  pp_detect_agents?: () => Promise<Array<JsonPayload>>;
  pp_configure_agent?: (agentId: string) => Promise<void>;
  pp_unconfigure_agent?: (agentId: string) => Promise<void>;
  pp_check_updates?: () => Promise<JsonPayload>;
  pp_download_update?: (url: string) => Promise<void>;
}

export const ProxyContext = createContext<ProxyContextType | null>(null)

export function useProxyContext() {
  const ctx = useContext(ProxyContext)
  if (!ctx) throw new Error('useProxyContext must be used within ProxyProvider')
  return ctx
}
