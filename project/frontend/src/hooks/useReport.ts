import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';
import { useOnlineStatus } from './useOnlineStatus';

const SAFE_MESSAGES: Record<string, string> = {
  REPORT_NOT_FOUND: 'Отчёт не найден.',
  ANALYSIS_NOT_FOUND: 'Анализ не найден.',
  AUTH_UNAVAILABLE: 'Сервис сессий временно недоступен.',
  SESSION_REQUIRED: 'Требуется вход в систему.',
  SESSION_EXPIRED: 'Сессия истекла. Войдите снова.',
  UPSTREAM_RATE_LIMITED: 'Сервис временно ограничил запросы. Повторите позже.',
  UPSTREAM_TIMEOUT: 'Сервис не ответил вовремя. Повторите позже.',
  NETWORK_ERROR: 'Нет соединения с сервером.',
  INTERNAL_ERROR: 'Сервис временно недоступен.',
};

function isAccessDenied(code: string): boolean {
  return (
    code === 'REPORT_NOT_FOUND' ||
    code === 'ANALYSIS_NOT_FOUND' ||
    code === 'FORBIDDEN' ||
    code === 'SESSION_REQUIRED' ||
    code === 'SESSION_EXPIRED' ||
    code === 'LOGIN_REQUIRED'
  );
}

function safeMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return SAFE_MESSAGES[error.code] ?? 'Не удалось загрузить отчёт.';
  }
  return 'Не удалось загрузить отчёт.';
}

export type ReportState<T = Report> =
  | { kind: 'idle' }
  | { kind: 'loading'; reportId: string }
  | { kind: 'loaded'; reportId: string; data: T; loadedAt: string }
  | { kind: 'error'; reportId: string; message: string }
  | { kind: 'offline'; reportId: string }
  | { kind: 'hidden'; reportId: string; code: string };

export function useReport<T = Report>(reportId: string | null | undefined) {
  const online = useOnlineStatus();
  const [state, setState] = useState<ReportState<T>>({ kind: 'idle' });
  const inFlightRef = useRef(false);
  const attemptRef = useRef(0);

  useEffect(() => {
    attemptRef.current += 1;
    inFlightRef.current = false;
    setState({ kind: 'idle' });
  }, [reportId]);

  const reload = useCallback(async () => {
    if (!reportId) return;
    if (inFlightRef.current) return;

    if (!online) {
      setState({ kind: 'offline', reportId });
      return;
    }

    inFlightRef.current = true;
    const attempt = ++attemptRef.current;
    const controller = new AbortController();

    setState({ kind: 'loading', reportId });

    try {
      const data = await repositoryService.getReport(reportId, controller.signal);
      if (attemptRef.current !== attempt) return;
      setState({
        kind: 'loaded',
        reportId,
        data: data as T,
        loadedAt: new Date().toISOString(),
      });
    } catch (error) {
      if (attemptRef.current !== attempt) return;
      const code = error instanceof ApiError ? error.code : 'INTERNAL_ERROR';
      if (isAccessDenied(code)) {
        setState({ kind: 'hidden', reportId, code });
      } else {
        setState({ kind: 'error', reportId, message: safeMessage(error) });
      }
    } finally {
      if (attemptRef.current === attempt) inFlightRef.current = false;
    }
  }, [reportId, online]);

  return {
    state,
    reload,
    isOffline: !online,
  };
}