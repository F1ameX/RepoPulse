import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { AnalysisStatus } from '@/types';

export interface ConfirmedStatus extends AnalysisStatus {
  reportId: string | null;
  reasonCode: string | null;
  updatedAt: string;
}
export type AnalysisStatusState =
  | { kind: 'idle' }
  | { kind: 'loading' | 'offline'; analysisId: string; lastConfirmed: ConfirmedStatus | null }
  | { kind: 'confirmed'; analysisId: string; lastConfirmed: ConfirmedStatus }
  | { kind: 'error'; analysisId: string; message: string; lastConfirmed: ConfirmedStatus | null }
  | { kind: 'hidden'; analysisId: string; code: string };

export function useAnalysisStatus(analysisId: string | null | undefined) {
  const [state, setState] = useState<AnalysisStatusState>({ kind: 'idle' });
  const refreshRef = useRef<() => Promise<void>>(async () => {});

  useEffect(() => {
    const pollInterval = 5_000;

    let active = true;
    let pending = false;
    let stopped = false;
    let last: ConfirmedStatus | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | undefined;

    const refresh = async () => {
      if (!active || !analysisId || pending || stopped) return;

      clearTimeout(timer);

      pending = true;
      controller = new AbortController();

      setState({ kind: 'loading', analysisId, lastConfirmed: last });

      try {
        const data = await repositoryService.getAnalysis(analysisId, controller.signal);

        if (!active) return;

        if (
          data.analysis_id !== analysisId ||
          !['queued', 'running', 'completed', 'failed'].includes(data.status) ||
          !data.repository ||
          (data.status === 'completed' && !data.report_id)
        ) {
          throw new Error('Invalid status response');
        }
        last = {
          ...data,
          reportId: data.report_id,
          reasonCode: data.error?.code ?? null,
          updatedAt: new Date().toISOString(),
        };
        stopped = data.status === 'completed' || data.status === 'failed';
        setState({ kind: 'confirmed', analysisId, lastConfirmed: last });
      } catch (error) {
        if (!active) return;

        const code = error instanceof ApiError ? error.code : 'NETWORK_ERROR';

        if (
          [
            'SESSION_REQUIRED',
            'SESSION_EXPIRED',
            'LOGIN_REQUIRED',
            'ANALYSIS_NOT_FOUND',
            'FORBIDDEN',
          ].includes(code)
        ) {
          stopped = true;
          last = null;
          setState({ kind: 'hidden', analysisId, code });
        } else
          setState({
            kind: 'error',
            analysisId,
            message: 'Не удалось обновить статус',
            lastConfirmed: last,
          });
      } finally {
        pending = false;

        if (active && !stopped && navigator.onLine && document.visibilityState !== 'hidden') {
          timer = setTimeout(() => void refresh(), pollInterval);
        }
      }
    };
    const resume = () => {
      clearTimeout(timer);
      if (!navigator.onLine) {
        if (analysisId && !stopped && !pending)
          setState({ kind: 'offline', analysisId, lastConfirmed: last });
      } else if (document.visibilityState !== 'hidden') void refresh();
    };
    refreshRef.current = refresh;

    window.addEventListener('online', resume);
    window.addEventListener('offline', resume);
    document.addEventListener('visibilitychange', resume);

    queueMicrotask(() => {
      if (active) resume();
    });

    return () => {
      active = false;
      controller?.abort();
      clearTimeout(timer);
      window.removeEventListener('online', resume);
      window.removeEventListener('offline', resume);
      document.removeEventListener('visibilitychange', resume);
    };
  }, [analysisId]);

  const refresh = useCallback(() => refreshRef.current(), []);

  return {
    state,
    refresh,
    isOffline: state.kind === 'offline',
    isPending: state.kind === 'loading',
  };
}
