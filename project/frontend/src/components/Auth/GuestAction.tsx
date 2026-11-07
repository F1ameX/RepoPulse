import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Message } from '@/components';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
import { COOKIE_REQUIRED, notifySessionChanged } from '@/lib/authNavigation';
import { useAuth } from './AuthContext';

export function GuestAction({ disabled = false }: { disabled?: boolean } = {}) {
  const auth = useAuth();
  const navigate = useNavigate();
  const active = useRef<AbortController | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => () => active.current?.abort(), []);

  const continueAsGuest = async () => {
    if (disabled || active.current) return;
    const request = new AbortController();
    active.current = request;
    const signal = AbortSignal.any([request.signal, AbortSignal.timeout(15_000)]);
    setPending(true);
    setError('');
    try {
      const context = await authService.getSessionContext(signal).catch((error: unknown) => {
        if (error instanceof ApiError && error.code === 'SESSION_EXPIRED') return null;
        throw error;
      });
      if (!context?.guest) await authService.createGuestSession(signal);
      if (!context || context.account) {
        if (context?.account && auth) {
          const confirmed = await authService.getSessionContext(signal);
          if (!confirmed.guest) {
            setError(COOKIE_REQUIRED);
            return;
          }
          if (await auth.logout()) navigate('/', { replace: true });
          return;
        }
        await authService.logout(signal);
      }
      const confirmed = await authService.getSessionContext(signal);
      if (request.signal.aborted) return;
      if (!confirmed.guest || confirmed.account) {
        setError(COOKIE_REQUIRED);
        return;
      }
      notifySessionChanged();
      navigate('/', { replace: true });
    } catch {
      if (!request.signal.aborted)
        setError('Не удалось открыть гостевую сессию. Повторите попытку');
    } finally {
      if (!request.signal.aborted) {
        active.current = null;
        setPending(false);
      }
    }
  };

  return (
    <>
      <Button
        variant="secondary"
        disabled={disabled}
        isLoading={pending}
        onClick={() => void continueAsGuest()}
      >
        Продолжить как гость
      </Button>
      {error && <Message variant="warning">{error}</Message>}
    </>
  );
}
