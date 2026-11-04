import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import type { CreateAnalysisResponse } from '@/types';
import { validateRepositoryUrl } from '@/lib/repositoryUrl';
import { safeMessage } from '@/lib/apiErrors';

const STORAGE_KEY = 'repopulse.pending-submission';

const UNKNOWN_AFTER_MS = 35_000;

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
  | {
      kind: 'temporary-error';
      requestId: string;
      originalUrl: string;
      message: string;
    }
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
  context?: string;
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

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    const restore = async () => {
      try {
        const payload = JSON.parse(
          sessionStorage.getItem(STORAGE_KEY) ?? 'null',
        ) as SubmissionPayload | null;
        if (
          !payload ||
          typeof payload.requestId !== 'string' ||
          typeof payload.url !== 'string' ||
          typeof payload.context !== 'string'
        )
          return;
        const session = await authService.getSessionContext(controller.signal);
        if (!active || payloadRef.current) return;
        if (JSON.stringify(session) !== payload.context || (!session.account && !session.guest)) {
          sessionStorage.removeItem(STORAGE_KEY);
          return;
        }
        payloadRef.current = payload;
        setState({
          kind: 'unknown',
          requestId: payload.requestId,
          originalUrl: payload.url,
          since: new Date().toISOString(),
        });
      } catch {
        /* A failed restore never automatically sends a new request. */
      }
    };
    void restore();
    return () => {
      active = false;
      controller.abort();
    };
  }, []);

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
    async (payload: SubmissionPayload, revalidate = false) => {
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
        attemptRef.current += 1;
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
        const session = await authService.getSessionContext(controller.signal);

        if (attemptRef.current !== attempt) return;

        const context = JSON.stringify(session);

        if (
          (!session.account && !session.guest) ||
          (payload.context && payload.context !== context)
        )
          throw new ApiError('SESSION_EXPIRED', '');
        payload.context = context;

        try {
          sessionStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
        } catch {
          /* Storage may be unavailable. */
        }
        if (revalidate) {
          const validation = validateRepositoryUrl(payload.url);

          if (validation.ok === false) throw new ApiError(validation.code, validation.message);

          await repositoryService.validateRepository(payload.url, controller.signal);

          if (attemptRef.current !== attempt) return;
        }
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
        try {
          sessionStorage.removeItem(STORAGE_KEY);
        } catch {
          /* Storage may be unavailable. */
        }
        onAcceptedRef.current?.(response);
      } catch (error) {
        if (attemptRef.current !== attempt) return;
        clearTimer();
        inFlightRef.current = false;

        const code = error instanceof ApiError ? error.code : 'INTERNAL_ERROR';
        const message = safeMessage(error);

        if (code === 'NETWORK_ERROR' || code === 'SUBMISSION_PENDING') {
          setState({
            kind: 'unknown',
            requestId: payload.requestId,
            originalUrl: payload.url,
            since: new Date().toISOString(),
          });
        } else if (isTemporaryCode(code)) {
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
      void run({ requestId: crypto.randomUUID(), url }, true);
    },
    [run],
  );

  const reset = useCallback(() => {
    clearTimer();
    attemptRef.current += 1;
    inFlightRef.current = false;
    controllerRef.current?.abort();
    try {
      sessionStorage.removeItem(STORAGE_KEY);
    } catch {
      /* Storage may be unavailable. */
    }
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
