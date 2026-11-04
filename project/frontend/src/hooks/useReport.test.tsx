import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ApiError } from '@/services/api';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';
import { useReport } from './useReport';

const A1 = '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const A2 = '7f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const R1 = 'a1b2c3d4-1111-2222-3333-444444444444';
const R2 = 'b1b2c3d4-1111-2222-3333-444444444444';

const report: Report = {
  report_id: R1,
  analysis_id: A1,
  snapshot_id: 'snap-1',
  repository: {
    platform: 'github',
    github_id: '1',
    owner: 'owner',
    name: 'repo',
    canonical_url: 'https://github.com/owner/repo',
    default_branch: null,
    commit_sha: null,
  },
  assessment_at: '2026-10-29T09:00:00Z',
  window: { from: '2026-06-11T09:00:00Z', until: '2026-10-29T09:00:00Z' },
  completed_at: '2026-10-29T10:00:00Z',
  versions: {
    schema: 'v1', methodology: 'v1', scoring_config: 'v1', collector_profile: 'v1',
    secret_rules: 'v1', recommendation_catalog: 'v1', explanation_schema: 'v1', prompt: 'v1',
  },
  overall_score: 75,
  overall_reason: null,
  summary: 'summary',
  coverage: { calculated: 12, total: 12, percentage: 100, partial: 0, unavailable: 0, not_applicable: 0 },
  categories: [], sources: [], dependencies: [], findings: [], recommendations: [],
  explanation: {
    mode: 'deterministic_fallback', fallback_reason: null, model: null,
    catalog_version: 'v1', schema_version: 'v1', prompt_version: 'v1',
  },
};

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

it('S13/N07: loads R1 on mount and reloads the same reportId', async () => {
  const get = vi.spyOn(repositoryService, 'getReport').mockResolvedValue(report);
  const { result } = renderHook(() => useReport(R1));
  await waitFor(() => expect(result.current.state.kind).toBe('loaded'));
  await act(async () => { await result.current.reload(); });
  await waitFor(() => expect(get).toHaveBeenCalledTimes(2));
  expect(get.mock.calls[0][0]).toBe(R1);
  expect(get.mock.calls[1][0]).toBe(R1);
});

it('N07: read error keeps completed analysis intact and preserves data', async () => {
  vi.spyOn(repositoryService, 'getReport')
    .mockResolvedValueOnce(report)
    .mockRejectedValueOnce(new ApiError('INTERNAL_ERROR', ''));
  const { result } = renderHook(() => useReport(R1));
  await waitFor(() => expect(result.current.state.kind).toBe('loaded'));

  await act(async () => { await result.current.reload(); });
  await waitFor(() => expect(result.current.state.kind).toBe('error'));
  if (result.current.state.kind === 'error') {
    expect(result.current.state.data).toEqual(report);
    expect(result.current.state.loadedAt).toBeTruthy();
  }
});

it('N08: offline keeps the previously loaded data', async () => {
  vi.spyOn(repositoryService, 'getReport').mockResolvedValue(report);
  const online = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  const { result } = renderHook(() => useReport(R1));
  await waitFor(() => expect(result.current.state.kind).toBe('loaded'));

  online.mockReturnValue(false);
  act(() => { window.dispatchEvent(new Event('offline')); });
  await waitFor(() => expect(result.current.state.kind).toBe('offline'));
  if (result.current.state.kind === 'offline') {
    expect(result.current.state.data).toEqual(report);
    expect(result.current.state.loadedAt).toBeTruthy();
  }
});

it('N09: access denial erases the data and hides the report', async () => {
  vi.spyOn(repositoryService, 'getReport')
    .mockResolvedValueOnce(report)
    .mockRejectedValueOnce(new ApiError('SESSION_EXPIRED', ''));
  const { result } = renderHook(() => useReport(R1));
  await waitFor(() => expect(result.current.state.kind).toBe('loaded'));

  await act(async () => { await result.current.reload(); });
  await waitFor(() => expect(result.current.state.kind).toBe('hidden'));
  if (result.current.state.kind === 'hidden') {
    expect(result.current.state.code).toBe('SESSION_EXPIRED');
  }
});

it('N10: a late response after reportId change is ignored', async () => {
  let finish!: (v: Report) => void;
  const get = vi
    .spyOn(repositoryService, 'getReport')
    .mockReturnValueOnce(new Promise((r) => { finish = r; }))
    .mockResolvedValueOnce({ ...report, report_id: R2, analysis_id: A2 });

  const { result, rerender } = renderHook(({ id }: { id: string }) => useReport(id), {
    initialProps: { id: R1 },
  });
  await waitFor(() => expect(get).toHaveBeenCalledTimes(1));
  rerender({ id: R2 });
  await waitFor(() => expect(result.current.state.kind).toBe('loaded'));

  await act(async () => { finish(report); });
  if (result.current.state.kind === 'loaded') {
    expect(result.current.state.reportId).toBe(R2);
  }
});

it('aborts the request on unmount', async () => {
  const get = vi.spyOn(repositoryService, 'getReport').mockReturnValue(new Promise(() => {}));
  const view = renderHook(() => useReport(R1));
  await waitFor(() => expect(get).toHaveBeenCalled());
  const signal = get.mock.calls[0][1];
  view.unmount();
  expect(signal?.aborted).toBe(true);
});

it('treats a mismatched report_id as an error', async () => {
  vi.spyOn(repositoryService, 'getReport').mockResolvedValue({ ...report, report_id: 'wrong-id' });
  const { result } = renderHook(() => useReport(R1));
  await waitFor(() => expect(result.current.state.kind).toBe('error'));
});