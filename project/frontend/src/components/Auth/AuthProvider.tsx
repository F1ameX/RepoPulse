import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { ApiError } from '@/services/api';
import { authService, type SessionInfo } from '@/services/authService';
import {
  LOGOUT_PENDING,
  SESSION_CHANGED,
  SESSION_REVISION,
  notifySessionChanged,
} from '@/lib/authNavigation';
import { AuthContext } from './AuthContext';

function rememberLogoutPending(pending: boolean) {
  try {
    if (pending) sessionStorage.setItem(LOGOUT_PENDING, '1');
    else sessionStorage.removeItem(LOGOUT_PENDING);
  } catch {
    return;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<SessionInfo | null>(null);
  const [checking, setChecking] = useState(true);
  const [expired, setExpired] = useState(false);
  const [revision, setRevision] = useState(0);
  const [logoutState, setLogoutState] = useState<'idle' | 'pending' | 'error'>(() => {
    try {
      return sessionStorage.getItem(LOGOUT_PENDING) ? 'error' : 'idle';
    } catch {
      return 'idle';
    }
  });
  const pendingLogout = useRef(false);

  useEffect(() => {
    if (logoutState !== 'idle') return;
    let controller: AbortController | undefined;
    let expiry: ReturnType<typeof setTimeout> | undefined;
    const check = async () => {
      controller?.abort();
      clearTimeout(expiry);
      const request = new AbortController();
      controller = request;
      setChecking(true);
      try {
        const { account: current } = await authService.getSessionContext(request.signal);
        if (request.signal.aborted) return;
        const remaining = current ? Date.parse(current.expires_at) - Date.now() : 0;
        setAccount(remaining > 0 ? current : null);
        setExpired((previous) => (current ? remaining <= 0 : previous));
        if (remaining > 0) {
          expiry = setTimeout(
            () => {
              setAccount(null);
              setExpired(true);
              void check();
            },
            Math.min(remaining, 2_147_483_647),
          );
        }
      } catch (error) {
        if (request.signal.aborted) return;
        setAccount(null);
        if (error instanceof ApiError && error.code === 'SESSION_EXPIRED') setExpired(true);
      } finally {
        if (!request.signal.aborted) setChecking(false);
      }
    };
    const resume = () => {
      if (document.visibilityState !== 'hidden') void check();
    };
    const changed = () => {
      setAccount(null);
      setRevision((value) => value + 1);
      void check();
    };
    const storage = (event: StorageEvent) => {
      if (event.key === SESSION_REVISION) changed();
    };
    void check();
    window.addEventListener(SESSION_CHANGED, changed);
    window.addEventListener('storage', storage);
    window.addEventListener('focus', resume);
    document.addEventListener('visibilitychange', resume);
    return () => {
      controller?.abort();
      clearTimeout(expiry);
      window.removeEventListener(SESSION_CHANGED, changed);
      window.removeEventListener('storage', storage);
      window.removeEventListener('focus', resume);
      document.removeEventListener('visibilitychange', resume);
    };
  }, [logoutState]);

  const logout = useCallback(async () => {
    if (pendingLogout.current) return false;
    pendingLogout.current = true;
    setAccount(null);
    setLogoutState('pending');
    rememberLogoutPending(true);
    try {
      await authService.logout();
      rememberLogoutPending(false);
      setExpired(false);
      setLogoutState('idle');
      notifySessionChanged();
      return true;
    } catch {
      setLogoutState('error');
      return false;
    } finally {
      pendingLogout.current = false;
    }
  }, []);

  return (
    <AuthContext.Provider value={{ account, checking, expired, revision, logoutState, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
