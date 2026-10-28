import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { CreateAnalysisResponse } from '@/types';

const UNKNOWN_AFTER_MS = 35_000;

const SAFE_MESSAGES: Record<string, string> = {
  ANALYSIS_FAILED: 'Анализ завершился ошибкой. Запустите его заново.',
  ANALYSIS_NOT_FOUND: 'Анализ не найден.',
  AUTH_UNAVAILABLE: 'Сервис сессий временно недоступен. Повторите позже.',
  REPORT_NOT_FOUND: 'Отчёт не найден.',
  REPORT_SAVE_FAILED: 'Не удалось сохранить отчёт.',
  REPOSITORY_NOT_FOUND_OR_PRIVATE: 'Публичный репозиторий недоступен.',
  VALIDATION_FAILED: 'Сервер отклонил параметры запроса.',
  SESSION_REQUIRED: 'Требуется вход в систему.',
  SESSION_EXPIRED: 'Сессия истекла. Войдите снова.',
  UPSTREAM_RATE_LIMITED: 'Сервис временно ограничил запросы. Повторите позже.',
  UPSTREAM_TIMEOUT: 'Сервис не ответил вовремя. Повторите позже.',
  NETWORK_ERROR: 'Нет соединения с сервером.',
  INTERNAL_ERROR: 'Сервис временно недоступен.',
  CSRF_REJECTED: 'Запрос отклонён. Обновите страницу.',
};

function safeMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return SAFE_MESSAGES[error.code] ?? 'Не удалось выполнить операцию. Повторите позже.';
  }
  return 'Не удалось выполнить операцию. Повторите позже.';
}

function isTemporaryCode(code: string): boolean {
  return (
    code === 'UPSTREAM_RATE_LIMITED' ||
    code === 'UPSTREAM_TIMEOUT' ||
    code === 'NETWORK_ERROR' ||
    code === 'AUTH_UNAVAILABLE' ||
    code === 'INTERNAL_ERROR'
  );
}

export type SubmissionState =
  | { kind: 'idle' }
  | { kind: 'submitting'; requestId: string; originalUrl: string }
  | { kind: 'unknown'; requestId: string; originalUrl: string; since: string }
  | { kind: 'temporary-error'; requestId: string; originalUrl: string; message: string }
  | {
      kind: 'confirmed-error';
      requestId: string;
      originalUrl: string;
      message: string;
      code: string;
    }
  | {
      kind: 'accepted';
      requestId: string;
      originalUrl: string;
      analysisId: string;
      reportId: string | null;
    };

interface SubmissionPayload {
  requestId: string;
  url: string;
}

export interface UseAnalysisSubmissionOptions {
  onAccepted?: (response: CreateAnalysisResponse) => void;
}

export function useAnalysisSubmission(options?: UseAnalysisSubmissionOptions) {
  const [state, setState] = useState<SubmissionState>({ kind: 'idle' });

  const attemptRef = useRef(0);
  const inFlightRef = useRef(false);
  const timerRef = useRef<number | null>(null);
  const payloadRef = useRef<SubmissionPayload | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  const onAcceptedRef = useRef(options?.onAccepted);
  useEffect(() => {
    onAcceptedRef.current = options?.onAccepted;
  });

  const clearTimer = useCallback(() => {
    if (timerRef.current != null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  useEffect(
    () => () => {
      clearTimer();
      attemptRef.current += 1;
      controllerRef.current?.abort();
    },
    [clearTimer],
  );

  const run = useCallback(
    async (payload: SubmissionPayload) => {
      if (inFlightRef.current) return;

      inFlightRef.current = true;
      clearTimer();
      const attempt = ++attemptRef.current;
      payloadRef.current = payload;

      controllerRef.current?.abort();
      const controller = new AbortController();
      controllerRef.current = controller;

      setState({
        kind: 'submitting',
        requestId: payload.requestId,
        originalUrl: payload.url,
      });

      timerRef.current = window.setTimeout(() => {
        if (attemptRef.current !== attempt) return;
        controller.abort();
        inFlightRef.current = false;
        setState({
          kind: 'unknown',
          requestId: payload.requestId,
          originalUrl: payload.url,
          since: new Date().toISOString(),
        });
      }, UNKNOWN_AFTER_MS);

      try {
        const response = await repositoryService.createAnalysis(
          {
            repository_url: payload.url,
            request_id: payload.requestId,
          },
          controller.signal,
        );

        if (attemptRef.current !== attempt) return;
        clearTimer();
        inFlightRef.current = false;

        setState({
          kind: 'accepted',
          requestId: payload.requestId,
          originalUrl: payload.url,
          analysisId: response.analysis_id,
          reportId: response.report_id ?? null,
        });
        onAcceptedRef.current?.(response);
      } catch (error) {
        if (attemptRef.current !== attempt) return;
        clearTimer();
        inFlightRef.current = false;

        const code = error instanceof ApiError ? error.code : 'INTERNAL_ERROR';
        const message = safeMessage(error);

        if (isTemporaryCode(code)) {
          setState({
            kind: 'temporary-error',
            requestId: payload.requestId,
            originalUrl: payload.url,
            message,
          });
        } else {
          setState({
            kind: 'confirmed-error',
            requestId: payload.requestId,
            originalUrl: payload.url,
            message,
            code,
          });
        }
      }
    },
    [clearTimer],
  );

  const submit = useCallback(
    (url: string) => {
      if (state.kind === 'submitting') return;
      void run({ requestId: crypto.randomUUID(), url });
    },
    [run, state.kind],
  );

  const retrySame = useCallback(() => {
    const payload = payloadRef.current;
    if (!payload) return;
    void run(payload);
  }, [run]);

  const retryNew = useCallback(
    (url: string) => {
      void run({ requestId: crypto.randomUUID(), url });
    },
    [run],
  );

  const reset = useCallback(() => {
    clearTimer();
    attemptRef.current += 1;
    inFlightRef.current = false;
    controllerRef.current?.abort();
    payloadRef.current = null;
    setState({ kind: 'idle' });
  }, [clearTimer]);

  const originalUrl =
    state.kind === 'unknown' ||
    state.kind === 'temporary-error' ||
    state.kind === 'confirmed-error' ||
    state.kind === 'accepted'
      ? state.originalUrl
      : null;

  return {
    state,
    submit,
    retrySame,
    retryNew,
    reset,
    originalUrl,
    hasUnconfirmed: state.kind === 'unknown' || state.kind === 'temporary-error',
    isPending: state.kind === 'submitting',
  };
}