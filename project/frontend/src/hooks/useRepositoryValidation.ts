import { useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import { URL_MESSAGES, validateRepositoryUrl } from '@/lib/repositoryUrl';

export type ValidationState =
  | { kind: 'idle' }
  | { kind: 'pending' }
  | { kind: 'field-error'; message: string }
  | { kind: 'unavailable' }
  | { kind: 'temporary-error'; message: string }
  | { kind: 'session-error'; message: string }
  | { kind: 'validated'; movedTo: string | null };

const CHECK_FAILED = 'Не удалось проверить доступность репозитория. Повторите попытку позже';

export function useRepositoryValidation() {
  const [url, setUrlState] = useState('');
  const [state, setState] = useState<ValidationState>({ kind: 'idle' });
  const inputRef = useRef<HTMLInputElement>(null);
  const activeRequest = useRef<AbortController | null>(null);

  useEffect(() => () => activeRequest.current?.abort(), []);
  useEffect(() => {
    if (state.kind === 'field-error') inputRef.current?.focus();
  }, [state]);

  const run = async () => {
    if (activeRequest.current) return;
    const checked = validateRepositoryUrl(url);
    if (!checked.ok) {
      setState({ kind: 'field-error', message: checked.message });
      inputRef.current?.focus();
      return;
    }

    const controller = new AbortController();
    activeRequest.current = controller;
    setState({ kind: 'pending' });
    try {
      const session = await authService.getSessionContext(controller.signal);
      if (controller.signal.aborted) return;
      if (!session.account && !session.guest) {
        await authService.createGuestSession(controller.signal);
        if (controller.signal.aborted) return;
        const confirmed = await authService.getSessionContext(controller.signal);
        if (controller.signal.aborted) return;
        if (!confirmed.account && !confirmed.guest) {
          setState({
            kind: 'session-error',
            message: 'Для работы RepoPulse разрешите cookie для этого сайта',
          });
          return;
        }
      }
      // Preserve the raw URL so server normalization runs exactly once.
      const validated = await repositoryService.validateRepository(url, controller.signal);
      if (controller.signal.aborted) return;
      setState({
        kind: 'validated',
        movedTo: validated.moved_from
          ? validated.repository.owner + '/' + validated.repository.name
          : null,
      });
      // C1 stops at preflight; job creation and its status page belong to C2.
    } catch (err) {
      if (controller.signal.aborted) return;
      if (err instanceof ApiError && Object.hasOwn(URL_MESSAGES, err.code)) {
        setState({
          kind: 'field-error',
          message: URL_MESSAGES[err.code as keyof typeof URL_MESSAGES],
        });
      } else if (err instanceof ApiError && err.code === 'REPOSITORY_NOT_FOUND_OR_PRIVATE') {
        setState({ kind: 'unavailable' });
      } else if (
        err instanceof ApiError &&
        ['SESSION_EXPIRED', 'SESSION_REQUIRED'].includes(err.code)
      ) {
        setState({ kind: 'session-error', message: 'Сессия истекла. Войдите снова' });
      } else {
        // Do not render arbitrary response messages or transport details.
        setState({ kind: 'temporary-error', message: CHECK_FAILED });
      }
    } finally {
      if (activeRequest.current === controller) activeRequest.current = null;
    }
  };

  return {
    url,
    state,
    inputRef,
    isPending: state.kind === 'pending',
    setUrl: (value: string) => {
      if (activeRequest.current) return;
      setUrlState(value);
      setState({ kind: 'idle' });
    },
    submit: () => {
      void run();
    },
    retry: () => {
      void run();
    },
    fix: () => {
      if (activeRequest.current) return;
      setState({ kind: 'idle' });
      inputRef.current?.focus();
    },
  };
}
