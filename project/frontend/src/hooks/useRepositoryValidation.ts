import { useRef, useState } from 'react';
import type { RefObject } from 'react';
import { ApiError } from '@/services/api';
import {
  repositoryService,
  classifyRepositoryError,
  normalizeUrl,
} from '@/services/repositoryService';

export type ValidationState =
  | { kind: 'idle' }
  | { kind: 'pending' }
  | { kind: 'permanent-error'; message: string }
  | { kind: 'temporary-error'; message: string }
  | { kind: 'moved'; newUrl: string };

const DEFAULT_MESSAGES: Record<string, string> = {
  INVALID_REPOSITORY_URL: 'Ссылка не похожа на GitHub или GitLab.',
  REPOSITORY_NOT_FOUND_OR_PRIVATE: 'Репозиторий не найден или приватный.',
  UNSUPPORTED_PLATFORM: 'Поддерживаются только GitHub и GitLab.',
  UPSTREAM_RATE_LIMITED:
    'GitHub временно ограничил запросы. Подождите и повторите.',
  UPSTREAM_TIMEOUT: 'GitHub не ответил вовремя.',
  NETWORK_ERROR: 'Сервер недоступен. Проверьте соединение.',
};

export interface UseRepositoryValidationOptions {
  /** Вызывается, когда анализ создан. Обычно — redirect на /analysis/:id */
  onAnalysisCreated: (analysisId: string) => void;
  /** Переопределения сообщений по коду ошибки */
  messages?: Record<string, string>;
}

export interface UseRepositoryValidationResult {
  url: string;
  state: ValidationState;
  isPending: boolean;
  inputRef: React.RefObject<HTMLInputElement | null>;
  setUrl: (value: string) => void;
  submit: () => void;
  retry: () => void;
  fix: () => void;
  useNew: () => void;
  reset: () => void;
}

export function useRepositoryValidation({
  onAnalysisCreated,
  messages,
}: UseRepositoryValidationOptions): UseRepositoryValidationResult {
  const [url, setUrlState] = useState('');
  const [state, setState] = useState<ValidationState>({ kind: 'idle' });
  const inputRef = useRef<HTMLInputElement>(null);
  const inFlightRef = useRef(false);

  const isPending = state.kind === 'pending';
  const mergedMessages = { ...DEFAULT_MESSAGES, ...messages };

  const run = async (value: string) => {
    if (inFlightRef.current) return;
    const trimmed = value.trim();

    if (!trimmed) {
      setState({
        kind: 'permanent-error',
        message: 'Введите ссылку на репозиторий',
      });
      inputRef.current?.focus();
      return;
    }

    inFlightRef.current = true;
    setState({ kind: 'pending' });

    try {
      const validated = await repositoryService.validateRepository(trimmed);

      if (normalizeUrl(validated.canonical_url) !== normalizeUrl(trimmed)) {
        setState({ kind: 'moved', newUrl: validated.canonical_url });
        return;
      }

      const { analysis_id } = await repositoryService.createAnalysis({
        canonical_url: validated.canonical_url,
        request_id: crypto.randomUUID(),
      });
      onAnalysisCreated(analysis_id);
    } catch (err) {
      if (err instanceof ApiError) {
        const message =
          mergedMessages[err.code] ?? err.message ?? 'Неизвестная ошибка';
        const kind = classifyRepositoryError(err.code);
        setState(
          kind === 'temporary'
            ? { kind: 'temporary-error', message }
            : { kind: 'permanent-error', message },
        );
      } else {
        setState({ kind: 'permanent-error', message: 'Неизвестная ошибка' });
      }
    } finally {
      inFlightRef.current = false;
    }
  };

  return {
    url,
    state,
    isPending,
    inputRef,
    setUrl: (value) => {
      setUrlState(value);
      if (state.kind !== 'pending' && state.kind !== 'idle') {
        setState({ kind: 'idle' });
      }
    },
    submit: () => {
      if (inFlightRef.current) return;
      void run(url);
    },
    retry: () => {
      if (inFlightRef.current) return;
      void run(url);
    },
    fix: () => {
      setState({ kind: 'idle' });
      inputRef.current?.focus();
      inputRef.current?.select();
    },
    useNew: () => {
      if (state.kind !== 'moved' || inFlightRef.current) return;
      setUrlState(state.newUrl);
      void run(state.newUrl);
    },
    reset: () => setState({ kind: 'idle' }),
  };
}