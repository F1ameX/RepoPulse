import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import { useAnalysisSubmission } from './useAnalysisSubmission';

const A1 = '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const URL_UNDER_TEST = 'https://github.com/owner/repo';
const guest = { kind: 'guest' as const, login: null, expires_at: '2099-01-01T00:00:00Z' };
const okResponse = {
  outcome: 'accepted' as const,
  analysis_id: A1,
  report_id: null,
  status: 'queued' as const,
  repository: {
    platform: 'github' as const,
    github_id: '1',
    owner: 'owner',
    name: 'repo',
    canonical_url: 'https://github.com/owner/repo',
    default_branch: 'main',
    commit_sha: null,
  },
};

beforeEach(() => {
  vi.useFakeTimers();
  sessionStorage.clear();
  vi.spyOn(authService, 'getSessionContext').mockResolvedValue({ account: null, guest });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const flush = () => vi.advanceTimersByTimeAsync(0);

async function submitAndIdle(result: { current: ReturnType<typeof useAnalysisSubmission> }) {
  await act(async () => {
    result.current.submit(URL_UNDER_TEST);
    await flush();
    await flush();
  });
}

async function advanceToUnknown(result: { current: ReturnType<typeof useAnalysisSubmission> }) {
  await act(async () => { await vi.advanceTimersByTimeAsync(35_000); });
  expect(result.current.state.kind).toBe('unknown');
}

describe('FR-03 P01–P11', () => {
  it('P01: submitting blocks repeated submit', async () => {
    const create = vi
      .spyOn(repositoryService, 'createAnalysis')
      .mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    expect(result.current.state.kind).toBe('submitting');
    expect(result.current.isPending).toBe(true);
    await act(async () => {
      result.current.submit(URL_UNDER_TEST);
      await flush();
    });
    expect(create).toHaveBeenCalledTimes(1);
  });

  it('P02: a double click produces one POST', async () => {
    const create = vi
      .spyOn(repositoryService, 'createAnalysis')
      .mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useAnalysisSubmission());
    await act(async () => {
      result.current.submit(URL_UNDER_TEST);
      result.current.submit(URL_UNDER_TEST);
      await flush();
    });
    expect(create).toHaveBeenCalledTimes(1);
  });

  it('P03: after 35s becomes unknown and does not auto retry', async () => {
    const create = vi
      .spyOn(repositoryService, 'createAnalysis')
      .mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await advanceToUnknown(result);
    expect(create).toHaveBeenCalledTimes(1);
    expect(result.current.hasUnconfirmed).toBe(true);
  });

  it('P04: restores the same request ID after F5', async () => {
    vi.spyOn(repositoryService, 'createAnalysis').mockReturnValue(new Promise(() => {}));
    const first = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(first.result);
    await advanceToUnknown(first.result);
    const firstId = (first.result.current.state as { requestId: string }).requestId;
    first.unmount();

    const second = renderHook(() => useAnalysisSubmission());
    await act(async () => {
      await flush();
      await flush();
    });
    expect(second.result.current.state.kind).toBe('unknown');
    expect((second.result.current.state as { requestId: string }).requestId).toBe(firstId);
  });

  it('P05: retrySame sends the same request_id', async () => {
    const create = vi
      .spyOn(repositoryService, 'createAnalysis')
      .mockReturnValueOnce(new Promise(() => {}))
      .mockReturnValueOnce(new Promise(() => {}));
    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await advanceToUnknown(result);
    const firstId = create.mock.calls[0][0].request_id;
    await act(async () => {
      result.current.retrySame();
      await flush();
      await flush();
    });
    expect(create.mock.calls[1][0].request_id).toBe(firstId);
  });

  it('P07: retryNew generates a new request_id and revalidates URL', async () => {
    vi.spyOn(repositoryService, 'createAnalysis').mockReturnValue(new Promise(() => {}));
    const validate = vi
      .spyOn(repositoryService, 'validateRepository')
      .mockResolvedValue({ repository: okResponse.repository, moved_from: null });
    const create = vi.mocked(repositoryService.createAnalysis);

    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await advanceToUnknown(result);
    const first = create.mock.calls[0][0].request_id;
    await act(async () => {
      result.current.retryNew(URL_UNDER_TEST);
      await flush();
      await flush();
      await flush();
    });
    expect(validate).toHaveBeenCalledTimes(1);
    expect(create.mock.calls[1][0].request_id).not.toBe(first);
  });

  it('P08: a late response of the previous attempt does not override the new one', async () => {
    let finishOld!: (v: typeof okResponse) => void;
    vi.spyOn(repositoryService, 'createAnalysis')
      .mockReturnValueOnce(new Promise((r) => { finishOld = r; }))
      .mockReturnValueOnce(new Promise(() => {}));
    vi.spyOn(repositoryService, 'validateRepository').mockResolvedValue({
      repository: okResponse.repository,
      moved_from: null,
    });

    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await advanceToUnknown(result);
    await act(async () => {
      result.current.retryNew(URL_UNDER_TEST);
      await flush();
      await flush();
      await flush();
    });
    expect(result.current.state.kind).toBe('submitting');
    await act(async () => { finishOld(okResponse); });
    expect(result.current.state.kind).toBe('submitting');
  });

  it('P10: SUBMISSION_PENDING is unknown, not failed', async () => {
    vi.spyOn(repositoryService, 'createAnalysis').mockRejectedValue(
      new ApiError('SUBMISSION_PENDING', 'Исходная отправка ещё проверяется'),
    );
    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await act(async () => { await flush(); });
    expect(result.current.state.kind).toBe('unknown');
  });

  // NOTE: FR-03 P05 требует «Запрос изменён. Отправьте его заново» (D1, #18480).
  it('P05: SUBMISSION_CHANGED currently shows the generic safe message', async () => {
    vi.spyOn(repositoryService, 'createAnalysis').mockRejectedValue(
      new ApiError('SUBMISSION_CHANGED', ''),
    );
    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await act(async () => { await flush(); });
    expect(result.current.state.kind).toBe('confirmed-error');
    expect((result.current.state as { message: string }).message).toBe(
      'Не удалось выполнить операцию. Повторите позже.',
    );
  });

  it('aborts the pending request on unmount', async () => {
    const create = vi
      .spyOn(repositoryService, 'createAnalysis')
      .mockReturnValue(new Promise(() => {}));
    const view = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(view.result);
    const signal = create.mock.calls[0][1];
    view.unmount();
    expect(signal?.aborted).toBe(true);
  });

  it('does not restore the pending request after the session context changed', async () => {
    vi.spyOn(repositoryService, 'createAnalysis').mockReturnValue(new Promise(() => {}));
    const first = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(first.result);
    first.unmount();

    vi.mocked(authService.getSessionContext).mockResolvedValue({ account: null, guest: null });
    const second = renderHook(() => useAnalysisSubmission());
    await act(async () => {
      await flush();
      await flush();
    });
    expect(second.result.current.state.kind).toBe('idle');
    expect(sessionStorage.getItem('repopulse.pending-submission')).toBeNull();
  });

  it.each([
    ['NETWORK_ERROR', 'unknown'],
    ['UPSTREAM_RATE_LIMITED', 'temporary-error'],
    ['UPSTREAM_TIMEOUT', 'temporary-error'],
    ['AUTH_UNAVAILABLE', 'temporary-error'],
    ['INTERNAL_ERROR', 'temporary-error'],
    ['ANALYSIS_FAILED', 'confirmed-error'],
    ['VALIDATION_FAILED', 'confirmed-error'],
    ['REPOSITORY_NOT_FOUND_OR_PRIVATE', 'confirmed-error'],
  ] as const)('classifies %s as %s', async (code, expected) => {
    vi.spyOn(repositoryService, 'createAnalysis').mockRejectedValue(new ApiError(code, ''));
    const { result } = renderHook(() => useAnalysisSubmission());
    await submitAndIdle(result);
    await act(async () => { await flush(); });
    expect(result.current.state.kind).toBe(expected);
  });
});