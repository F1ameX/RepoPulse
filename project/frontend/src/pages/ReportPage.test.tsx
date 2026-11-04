import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';
import ReportPage from './ReportPage';

const A1 = '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const R1 = 'a1b2c3d4-1111-2222-3333-444444444444';
const guest = { kind: 'guest' as const, login: null, expires_at: '2099-01-01T00:00:00Z' };

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
  overall_score: null, overall_reason: null, summary: '',
  coverage: { calculated: 0, total: 0, percentage: 0, partial: 0, unavailable: 0, not_applicable: 0 },
  categories: [], sources: [], dependencies: [], findings: [], recommendations: [],
  explanation: {
    mode: 'deterministic_fallback', fallback_reason: null, model: null,
    catalog_version: 'v1', schema_version: 'v1', prompt_version: 'v1',
  },
};

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/report/:id" element={<ReportPage />} />
      </Routes>
    </MemoryRouter>,
  );

beforeEach(() => {
  vi.spyOn(authService, 'getSessionContext').mockResolvedValue({ account: null, guest });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

it('invalid UUID → «Отчёт недоступен»', () => {
  renderAt('/report/not-a-uuid');
  expect(screen.getByRole('alert').textContent).toContain('Отчёт недоступен');
  expect(screen.getByRole('link', { name: 'Новый анализ' }).getAttribute('href')).toBe('/');
});

it('renders the fetched report', async () => {
  vi.spyOn(repositoryService, 'getReport').mockResolvedValue(report);
  renderAt(`/report/${R1}`);
  // Ищем уникальное поле "Анализ:" — owner/repo встречается и в заголовке, и в dd,
  // и в ссылке, поэтому по нему getByText падает на multiple.
  await screen.findByText(A1);
  expect(screen.getByRole('region', { name: 'Шапка отчёта' })).toBeTruthy();
});

it('N07: shows "Не удалось загрузить отчёт" with retry', async () => {
  const { ApiError } = await import('@/services/api');
  vi.spyOn(repositoryService, 'getReport').mockRejectedValue(new ApiError('INTERNAL_ERROR', ''));
  renderAt(`/report/${R1}`);
  await screen.findByText('Не удалось загрузить отчёт');
  expect(screen.getByRole('button', { name: 'Повторить загрузку' })).toBeTruthy();
});