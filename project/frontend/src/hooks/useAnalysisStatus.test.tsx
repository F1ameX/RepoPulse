import { act, cleanup, renderHook } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { repositoryService } from '@/services/repositoryService';
import type { AnalysisStatus } from '@/types';
import { useAnalysisStatus } from './useAnalysisStatus';

const status: AnalysisStatus = {
  analysis_id: '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef',
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

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
  vi.useRealTimers();
});

it('waits five seconds after each response, prevents parallel reads and stops on terminal status', async () => {
  vi.useFakeTimers();
  vi.stubEnv('VITE_ANALYSIS_POLL_INTERVAL_MS', '1000');
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  let finish!: (value: AnalysisStatus) => void;
  const get = vi
    .spyOn(repositoryService, 'getAnalysis')
    .mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    )
    .mockResolvedValueOnce(status)
    .mockResolvedValueOnce({ ...status, status: 'failed' });
  const { result } = renderHook(() => useAnalysisStatus(status.analysis_id));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0);
  });
  expect(get).toHaveBeenCalledTimes(1);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(8000);
    void result.current.refresh();
  });
  expect(get).toHaveBeenCalledTimes(1);
  await act(async () => {
    finish(status);
  });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(4999);
  });
  expect(get).toHaveBeenCalledTimes(1);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  expect(get).toHaveBeenCalledTimes(2);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(4999);
  });
  expect(get).toHaveBeenCalledTimes(2);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  expect(get).toHaveBeenCalledTimes(3);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(10000);
  });
  expect(get).toHaveBeenCalledTimes(3);
});
