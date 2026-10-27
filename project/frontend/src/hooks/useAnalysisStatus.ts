import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { AnalysisStatusValue } from '@/services/repositoryService';
import { useOnlineStatus } from './useOnlineStatus';

const SAFE_MESSAGES: Record<string, string> = {
  ANALYSIS_NOT_FOUND: 'Анализ не найден.',
  SESSION_REQUIRED: 'Требуется вход в систему.',
  SESSION_EXPIRED: 'Сессия истекла. Войдите снова.',
  UPSTREAM_RATE_LIMITED: 'Сервис временно ограничил запросы. Повторите позже.',
  UPSTREAM_TIMEOUT: 'Сервис не ответил вовремя. Повторите позже.',
  NETWORK_ERROR: 'Нет соединения с сервером.',
  INTERNAL_ERROR: 'Сервис временно недоступен.',
};

function safeMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return SAFE_MESSAGES[error.code] ?? 'Не удалось проверить статус анализа.';
  }
  return 'Не удалось проверить статус анализа.';
}

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
  | { kind: 'offline'; analysisId: string; lastConfirmed: ConfirmedStatus | null };

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
    setState({
      kind: 'loading',
      analysisId,
      lastConfirmed: lastConfirmedRef.current,
    });

    try {
      // === BACKEND CALL: GET /analyses/:id ===
      // Ожидаемая форма (совпадает с services/repositoryService.ts):
      // { analysis_id, status: 'queued'|'running'|'completed'|'failed',
      //   report_id?: string|null, error?: { code, message } | null }
      const data = await repositoryService.getAnalysis(analysisId);
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
      setState({
        kind: 'error',
        analysisId,
        message: safeMessage(error),
        lastConfirmed: lastConfirmedRef.current,
      });
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