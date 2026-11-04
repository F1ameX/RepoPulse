import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { AnalysisStatus } from '@/types';
import { useAnalysisStatus } from './useAnalysisStatus';

const A1 = '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const A2 = '7f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const R1 = 'a1b2c3d4-1111-2222-3333-444444444444';

const baseStatus: AnalysisStatus = {
  analysis_id: A1,
  repository: {
    platform: 'github',
    github_id: '1',
    owner: 'owner',
    name: 'repo',
    canonical_url: 'https://github.com/owner/repo',
    default_branch: null,
    commit_sha: null,
  },
  status: 'running',
  stage: 'collecting_git',
  report_id: null,
  error: null,
  created_at: '2026-10-29T10:00:00Z',
  started_at: '2026-10-29T10:00:01Z',
  completed_at: null,
};

const flush = () => vi.advanceTimersByTimeAsync(0);

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it('S03/S10: accepts the first terminal response and stops polling', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  const get = vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue({
    ...baseStatus,
    status: 'completed',
    report_id: R1,
  });
  const { result } = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  expect(result.current.state.kind).toBe('confirmed');
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(get).toHaveBeenCalledTimes(1);
});

it('S05: polls 5 seconds after the previous response, never in parallel', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  let finish!: (v: AnalysisStatus) => void;
  const get = vi
    .spyOn(repositoryService, 'getAnalysis')
    .mockReturnValueOnce(new Promise((r) => { finish = r; }))
    .mockResolvedValueOnce(baseStatus);
  const { result } = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  expect(get).toHaveBeenCalledTimes(1);

  await act(async () => {
    void result.current.refresh();
    await vi.advanceTimersByTimeAsync(8_000);
  });
  expect(get).toHaveBeenCalledTimes(1);

  await act(async () => { finish(baseStatus); });
  await act(async () => { await vi.advanceTimersByTimeAsync(4_999); });
  expect(get).toHaveBeenCalledTimes(1);
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  expect(get).toHaveBeenCalledTimes(2);
});

it('S06: a read error keeps lastConfirmed and does not become failed', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  vi.spyOn(repositoryService, 'getAnalysis')
    .mockResolvedValueOnce(baseStatus)
    .mockRejectedValueOnce(new ApiError('INTERNAL_ERROR', ''));
  const { result } = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  expect(result.current.state.kind).toBe('confirmed');

  await act(async () => {
    await result.current.refresh();
    await flush();
  });
  expect(result.current.state.kind).toBe('error');
  if (result.current.state.kind === 'error') {
    expect(result.current.state.lastConfirmed?.status).toBe('running');
  }
});

it('S08: hidden tab does not poll, visible triggers one request', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  const vis = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
  const get = vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue(baseStatus);
  const { result } = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  expect(get).toHaveBeenCalledTimes(0);
  expect(result.current.state.kind).toBe('idle');

  vis.mockReturnValue('visible');
  await act(async () => {
    document.dispatchEvent(new Event('visibilitychange'));
    await flush();
  });
  expect(get).toHaveBeenCalledTimes(1);
});

it('N04: offline pauses, online resumes with one request', async () => {
  vi.useFakeTimers();
  const online = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  const get = vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue(baseStatus);
  const { result } = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  const before = get.mock.calls.length;

  online.mockReturnValue(false);
  act(() => { window.dispatchEvent(new Event('offline')); });
  expect(result.current.state.kind).toBe('offline');
  if (result.current.state.kind === 'offline') {
    expect(result.current.state.lastConfirmed?.status).toBe('running');
  }

  online.mockReturnValue(true);
  await act(async () => {
    window.dispatchEvent(new Event('online'));
    await flush();
  });
  expect(get.mock.calls.length).toBe(before + 1);
});

it('S11: late response after analysisId change is ignored', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  let finishA!: (v: AnalysisStatus) => void;
  vi.spyOn(repositoryService, 'getAnalysis')
    .mockReturnValueOnce(new Promise((r) => { finishA = r; }))
    .mockResolvedValueOnce({ ...baseStatus, analysis_id: A2 });

  const { result, rerender } = renderHook(({ id }: { id: string }) => useAnalysisStatus(id), {
    initialProps: { id: A1 },
  });
  await act(async () => { await flush(); });
  rerender({ id: A2 });
  await act(async () => { await flush(); await flush(); });
  expect(result.current.state.kind).toBe('confirmed');

  await act(async () => { finishA({ ...baseStatus, status: 'completed', report_id: R1 }); });
  if (result.current.state.kind === 'confirmed') {
    expect(result.current.state.analysisId).toBe(A2);
  }
});

it('S12: denied access hides the analysis', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  vi.spyOn(repositoryService, 'getAnalysis')
    .mockResolvedValueOnce(baseStatus)
    .mockRejectedValueOnce(new ApiError('SESSION_EXPIRED', ''));
  const { result } = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  expect(result.current.state.kind).toBe('confirmed');

  await act(async () => {
    await result.current.refresh();
    await flush();
  });
  expect(result.current.state.kind).toBe('hidden');
  if (result.current.state.kind === 'hidden') {
    expect(result.current.state.code).toBe('SESSION_EXPIRED');
  }
});

it('aborts the request on unmount', async () => {
  vi.useFakeTimers();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  const get = vi.spyOn(repositoryService, 'getAnalysis').mockReturnValue(new Promise(() => {}));
  const view = renderHook(() => useAnalysisStatus(A1));
  await act(async () => { await flush(); });
  const signal = get.mock.calls[0][1];
  view.unmount();
  expect(signal?.aborted).toBe(true);
});