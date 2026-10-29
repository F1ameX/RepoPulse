import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';
import { safeMessage } from '@/lib/apiErrors';
import { useOnlineStatus } from './useOnlineStatus';

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

export type ReportState<T = Report> =
  | { kind: 'idle' }
  | { kind: 'loading'; reportId: string; data: T | null; loadedAt: string | null }
  | { kind: 'loaded'; reportId: string; data: T; loadedAt: string }
  | { kind: 'error'; reportId: string; message: string; data: T | null; loadedAt: string | null }
  | { kind: 'offline'; reportId: string; data: T | null; loadedAt: string | null }
  | { kind: 'hidden'; reportId: string; code: string };

export function useReport<T = Report>(reportId: string | null | undefined) {
  const online = useOnlineStatus();
  const [state, setState] = useState<ReportState<T>>({ kind: 'idle' });
  const inFlightRef = useRef(false);
  const attemptRef = useRef(0);
  const lastLoadedRef = useRef<{ data: T; loadedAt: string } | null>(null);

  useEffect(() => {
    attemptRef.current += 1;
    inFlightRef.current = false;
    lastLoadedRef.current = null;
    setState({ kind: 'idle' });
  }, [reportId]);

  const reload = useCallback(async () => {
    if (!reportId) return;
    if (inFlightRef.current) return;

    if (!online) {
      setState({
        kind: 'offline',
        reportId,
        data: lastLoadedRef.current?.data ?? null,
        loadedAt: lastLoadedRef.current?.loadedAt ?? null,
      });
      return;
    }

    inFlightRef.current = true;
    const attempt = ++attemptRef.current;
    const controller = new AbortController();

    setState({
      kind: 'loading',
      reportId,
      data: lastLoadedRef.current?.data ?? null,
      loadedAt: lastLoadedRef.current?.loadedAt ?? null,
    });

    try {
      const data = await repositoryService.getReport(reportId, controller.signal);
      if (attemptRef.current !== attempt) return;
      const loadedAt = new Date().toISOString();
      lastLoadedRef.current = { data: data as T, loadedAt };
      setState({ kind: 'loaded', reportId, data: data as T, loadedAt });
    } catch (error) {
      if (attemptRef.current !== attempt) return;
      const code = error instanceof ApiError ? error.code : 'INTERNAL_ERROR';
      if (isAccessDenied(code)) {
        lastLoadedRef.current = null;
        setState({ kind: 'hidden', reportId, code });
      } else {
        setState({
          kind: 'error',
          reportId,
          message: safeMessage(error, 'Не удалось загрузить отчёт.'),
          data: lastLoadedRef.current?.data ?? null,
          loadedAt: lastLoadedRef.current?.loadedAt ?? null,
        });
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