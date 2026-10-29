import { Fragment, useEffect, useRef, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Button, Message } from '@/components';
import { ApiError } from '@/services/api';
import { authService, type SessionContext } from '@/services/authService';
import { isSessionError } from '@/lib/analysisView';

export function AnalysisAccess({ children }: { children: ReactNode }) {
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
      clearTimeout(expiryTimer);
      setAccess((old) => ({ ...old, kind: 'checking' }));
      try {
        const session: SessionContext = await authService.getSessionContext(
          AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]),
        );
        if (!active || sequence !== current) return;
        if (!session.account && !session.guest) {
          setAccess({ kind: 'denied', context: null });
          return;
        }
        const context = JSON.stringify(session);
        setAccess({ kind: 'ready', context });
        const expiry = Math.min(
          ...[session.account, session.guest]
            .filter((x) => x !== null)
            .map((x) => Date.parse(x.expires_at)),
        );
        if (Number.isFinite(expiry) && expiry > Date.now())
          expiryTimer = setTimeout(
            () => void check(),
            Math.min(expiry - Date.now(), 2_147_483_647),
          );
      } catch (error) {
        if (!active || sequence !== current) return;
        if (error instanceof ApiError && isSessionError(error.code))
          setAccess({ kind: 'denied', context: null });
        else setAccess((old) => ({ ...old, kind: 'error' }));
      }
    };
    const resume = () => {
      if (document.visibilityState !== 'hidden') void check();
    };
    retryRef.current = () => void check();
    window.addEventListener('focus', resume);
    window.addEventListener('online', resume);
    window.addEventListener('storage', resume);
    window.addEventListener('repopulse:session-changed', resume);
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
      window.removeEventListener('storage', resume);
      window.removeEventListener('repopulse:session-changed', resume);
      document.removeEventListener('visibilitychange', resume);
    };
  }, []);
  return (
    <>
      {access.context && (
        <div hidden={access.kind !== 'ready'}>
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
        <Message variant="error" actions={<Link to="/">Продолжить как гость</Link>}>
          Сессия истекла. Войдите снова
        </Message>
      )}
    </>
  );
}
