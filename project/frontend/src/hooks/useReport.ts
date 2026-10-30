import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';

export type ReportState<T = Report> =
  | { kind: 'idle' }
  | { kind: 'loading' | 'offline'; reportId: string; data: T | null; loadedAt: string | null }
  | { kind: 'loaded'; reportId: string; data: T; loadedAt: string }
  | { kind: 'error'; reportId: string; message: string; data: T | null; loadedAt: string | null }
  | { kind: 'hidden'; reportId: string; code: string };

export function useReport(reportId: string | null | undefined) {
  const [state, setState] = useState<ReportState>({ kind: 'idle' });
  const reloadRef = useRef<() => Promise<void>>(async () => {});
  useEffect(() => {
    let active = true;
    let pending = false;
    let denied = false;
    let last: { data: Report; loadedAt: string } | null = null;
    let controller: AbortController | undefined;
    const reload = async () => {
      if (!active || !reportId || pending || denied) return;
      pending = true;
      controller = new AbortController();
      setState({
        kind: 'loading',
        reportId,
        data: last?.data ?? null,
        loadedAt: last?.loadedAt ?? null,
      });
      try {
        const data = await repositoryService.getReport(reportId, controller.signal);
        if (!active) return;
        if (data.report_id !== reportId || !data.repository || !data.versions || !data.window)
          throw new Error('Invalid report response');
        last = { data, loadedAt: new Date().toISOString() };
        setState({ kind: 'loaded', reportId, ...last });
      } catch (error) {
        if (!active) return;
        const code = error instanceof ApiError ? error.code : 'NETWORK_ERROR';
        if (
          [
            'REPORT_NOT_FOUND',
            'ANALYSIS_NOT_FOUND',
            'FORBIDDEN',
            'SESSION_REQUIRED',
            'SESSION_EXPIRED',
            'LOGIN_REQUIRED',
          ].includes(code)
        ) {
          denied = true;
          last = null;
          setState({ kind: 'hidden', reportId, code });
        } else
          setState({
            kind: 'error',
            reportId,
            message: 'Не удалось загрузить отчёт',
            data: last?.data ?? null,
            loadedAt: last?.loadedAt ?? null,
          });
      } finally {
        pending = false;
      }
    };
    const resume = () => {
      if (!active || !reportId || denied) return;
      if (!navigator.onLine) {
        if (!pending)
          setState({
            kind: 'offline',
            reportId,
            data: last?.data ?? null,
            loadedAt: last?.loadedAt ?? null,
          });
      } else if (document.visibilityState !== 'hidden') void reload();
    };
    reloadRef.current = reload;
    window.addEventListener('online', resume);
    window.addEventListener('offline', resume);
    document.addEventListener('visibilitychange', resume);
    queueMicrotask(() => {
      if (active) resume();
    });
    return () => {
      active = false;
      controller?.abort();
      window.removeEventListener('online', resume);
      window.removeEventListener('offline', resume);
      document.removeEventListener('visibilitychange', resume);
    };
  }, [reportId]);
  const reload = useCallback(() => reloadRef.current(), []);
  return {
    state,
    reload,
    isOffline: state.kind === 'offline',
    isPending: state.kind === 'loading',
  };
}
