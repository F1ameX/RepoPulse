import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { AnalysisStatusValue } from '@/services/repositoryService';
import { safeMessage } from '@/lib/apiErrors';
import { useOnlineStatus } from './useOnlineStatus';

export interface ConfirmedStatus {
  status: AnalysisStatusValue;
  reportId: string | null;
  reasonCode: string | null;
  updatedAt: string;
}

export type AnalysisStatusState =
  | { kind: 'idle' }
  | { kind: 'loading'; analysisId: string; lastConfirmed: ConfirmedStatus | null }
  | { kind: 'confirmed'; analysisId: string; lastConfirmed: ConfirmedStatus }
  | {
      kind: 'error';
      analysisId: string;
      message: string;
      lastConfirmed: ConfirmedStatus | null;
    }
  | { kind: 'offline'; analysisId: string; lastConfirmed: ConfirmedStatus | null }
  | { kind: 'hidden'; analysisId: string; code: string };

export function useAnalysisStatus(analysisId: string | null | undefined) {
  const online = useOnlineStatus();
  const [state, setState] = useState<AnalysisStatusState>({ kind: 'idle' });
  const lastConfirmedRef = useRef<ConfirmedStatus | null>(null);
  const inFlightRef = useRef(false);
  const attemptRef = useRef(0);

  useEffect(() => {
    attemptRef.current += 1;
    inFlightRef.current = false;
    lastConfirmedRef.current = null;
    setState({ kind: 'idle' });
  }, [analysisId]);

  const refresh = useCallback(async () => {
    if (!analysisId) return;
    if (inFlightRef.current) return;

    if (!online) {
      setState({
        kind: 'offline',
        analysisId,
        lastConfirmed: lastConfirmedRef.current,
      });
      return;
    }

    inFlightRef.current = true;
    const attempt = ++attemptRef.current;
    const controller = new AbortController();

    setState({
      kind: 'loading',
      analysisId,
      lastConfirmed: lastConfirmedRef.current,
    });

    try {
      const data = await repositoryService.getAnalysis(analysisId, controller.signal);
      if (attemptRef.current !== attempt) return;

      const confirmed: ConfirmedStatus = {
        status: data.status,
        reportId: data.report_id ?? null,
        reasonCode: data.error?.code ?? null,
        updatedAt: new Date().toISOString(),
      };
      lastConfirmedRef.current = confirmed;
      setState({ kind: 'confirmed', analysisId, lastConfirmed: confirmed });
    } catch (error) {
      if (attemptRef.current !== attempt) return;

      const code = error instanceof ApiError ? error.code : 'NETWORK_ERROR';

      if (['SESSION_REQUIRED', 'SESSION_EXPIRED', 'LOGIN_REQUIRED', 'ANALYSIS_NOT_FOUND', 'FORBIDDEN'].includes(code)) {
        lastConfirmedRef.current = null;
        setState({ kind: 'hidden', analysisId, code });
      } else {
        setState({
          kind: 'error',
          analysisId,
          message: safeMessage(error, 'Не удалось проверить статус анализа.'),
          lastConfirmed: lastConfirmedRef.current,
        });
      }
    } finally {
      if (attemptRef.current === attempt) inFlightRef.current = false;
    }
  }, [analysisId, online]);

  return {
    state,
    refresh,
    isOffline: !online,
  };
}