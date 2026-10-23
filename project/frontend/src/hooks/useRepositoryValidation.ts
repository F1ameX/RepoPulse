import { useRef, useState, type RefObject } from 'react';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
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
  REPOSITORY_CHECK_FAILED: 'Сервис проверки временно недоступен. Попробуйте позже.',
};

export interface UseRepositoryValidationOptions {
  onAnalysisCreated: (analysisId: string) => void;
  messages?: Record<string, string>;
}

export interface UseRepositoryValidationResult {
  url: string;
  state: ValidationState;
  isPending: boolean;
  inputRef: RefObject<HTMLInputElement | null>;
  setUrl: (value: string) => void;
  submit: () => void;
  retry: () => void;
  fix: () => void;
  useNew: () => void;
  reset: () => void;
}

const getStorageKey = (url: string) => `req_id_${normalizeUrl(url)}`;

const getOrCreateRequestId = (url: string) => {
  const key = getStorageKey(url);
  let id = sessionStorage.getItem(key);
  if (!id) {
    id = crypto.randomUUID();
    sessionStorage.setItem(key, id);
  }
  return id;
};

const clearRequestId = (url: string) => {
  sessionStorage.removeItem(getStorageKey(url));
};

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
      try {
        await authService.getSessionContext();
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          await authService.createGuestSession();
        } else {
          throw err;
        }
      }

      const validated = await repositoryService.validateRepository(trimmed);
      const canonicalUrl = validated.repository.canonical_url;

      if (normalizeUrl(canonicalUrl) !== normalizeUrl(trimmed)) {
        setState({ kind: 'moved', newUrl: canonicalUrl });
        return;
      }

      const requestId = getOrCreateRequestId(canonicalUrl);
      
      const { analysis_id } = await repositoryService.createAnalysis({
        repository_url: canonicalUrl, // Изменено на repository_url согласно контракту
        request_id: requestId,
      });
      
      clearRequestId(canonicalUrl);
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