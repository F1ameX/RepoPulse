import { Fragment, useEffect, useRef, useState, type ReactNode } from 'react';
import { useLocation } from 'react-router-dom';
import { Button, Message } from '@/components';
import { GuestAction } from '@/components/Auth/GuestAction';
import { ButtonLink } from '@/components/Button/ButtonLink';
import { ApiError } from '@/services/api';
import { authService, type SessionContext } from '@/services/authService';
import { reportReturnTo, SESSION_CHANGED, SESSION_REVISION } from '@/lib/authNavigation';
import { isSessionError } from '@/lib/analysisView';

export function AnalysisAccess({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const [access, setAccess] = useState<{
    kind: 'checking' | 'ready' | 'error' | 'denied';
    context: string | null;
  }>({ kind: 'checking', context: null });
  const retryRef = useRef<() => void>(() => {});
  useEffect(() => {
    let active = true;
    let controller: AbortController | null = null;
    let expiryTimer: ReturnType<typeof setTimeout> | undefined;
    let sequence = 0;
    const check = async () => {
      const current = ++sequence;
      controller?.abort();
      controller = new AbortController();
      setAccess((old) => ({ ...old, kind: 'checking' }));
      try {
        const session: SessionContext = await authService.getSessionContext(
          AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]),
        );
        if (!active || sequence !== current) return;
        clearTimeout(expiryTimer);
        if (!session.account && !session.guest) {
          setAccess({ kind: 'denied', context: null });
          return;
        }
        const expiry = Math.min(
          ...[session.account, session.guest]
            .filter((x) => x !== null)
            .map((x) => Date.parse(x.expires_at)),
        );
        const remaining = expiry - Date.now();
        if (!Number.isFinite(remaining) || remaining <= 0) {
          setAccess({ kind: 'denied', context: null });
          return;
        }
        setAccess({ kind: 'ready', context: JSON.stringify(session) });
        expiryTimer = setTimeout(() => changed(), Math.min(remaining, 2_147_483_647));
      } catch (error) {
        if (!active || sequence !== current) return;
        if (error instanceof ApiError && isSessionError(error.code)) {
          clearTimeout(expiryTimer);
          setAccess({ kind: 'denied', context: null });
        } else setAccess((old) => ({ ...old, kind: 'error' }));
      }
    };
    const resume = () => {
      if (document.visibilityState !== 'hidden') void check();
    };
    const changed = () => {
      clearTimeout(expiryTimer);
      setAccess({ kind: 'checking', context: null });
      void check();
    };
    const storage = (event: StorageEvent) => {
      if (event.key === SESSION_REVISION) changed();
    };
    retryRef.current = () => void check();
    window.addEventListener('focus', resume);
    window.addEventListener('online', resume);
    window.addEventListener('storage', storage);
    window.addEventListener(SESSION_CHANGED, changed);
    document.addEventListener('visibilitychange', resume);
    queueMicrotask(() => {
      if (active) void check();
    });
    return () => {
      active = false;
      controller?.abort();
      clearTimeout(expiryTimer);
      window.removeEventListener('focus', resume);
      window.removeEventListener('online', resume);
      window.removeEventListener('storage', storage);
      window.removeEventListener(SESSION_CHANGED, changed);
      document.removeEventListener('visibilitychange', resume);
    };
  }, []);
  return (
    <>
      {access.context && (
        <div inert={access.kind !== 'ready'} aria-busy={access.kind === 'checking'}>
          <Fragment key={access.context}>{children}</Fragment>
        </div>
      )}
      {access.kind === 'error' && (
        <Message
          variant="warning"
          actions={
            <Button variant="secondary" onClick={() => retryRef.current()}>
              Повторить проверку доступа
            </Button>
          }
        >
          Не удалось проверить сессию
        </Message>
      )}
      {access.kind === 'denied' && (
        <Message
          variant="error"
          actions={
            <>
              <ButtonLink
                to="/login"
                variant="secondary"
                state={{ returnTo: reportReturnTo(pathname) }}
              >
                Войти
              </ButtonLink>
              <GuestAction />
            </>
          }
        >
          Сессия истекла. Войдите снова
        </Message>
      )}
    </>
  );
}
