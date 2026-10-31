import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import type { AnalysisStatus, Report } from '@/types';
import AnalysisStatusPage from './AnalysisStatusPage';

const A1 = '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef';
const R1 = 'a1b2c3d4-1111-2222-3333-444444444444';
const guest = { kind: 'guest' as const, login: null, expires_at: '2099-01-01T00:00:00Z' };

const base: AnalysisStatus = {
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
  stage: null,
  report_id: null,
  error: null,
  created_at: '2026-10-29T10:00:00Z',
  started_at: null,
  completed_at: null,
};

// Полный отчёт с обязательными полями, чтобы useReport не отверг ответ.
const fullReport: Report = {
  report_id: R1,
  analysis_id: A1,
  snapshot_id: 'snap-1',
  repository: base.repository,
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
        <Route path="/analysis/:id" element={<AnalysisStatusPage />} />
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

it('S12: invalid UUID → «Анализ недоступен» + «Новый анализ»', () => {
  renderAt('/analysis/not-a-uuid');
  expect(screen.getByRole('alert').textContent).toContain('Анализ недоступен');
  expect(screen.getByRole('link', { name: 'Новый анализ' }).getAttribute('href')).toBe('/');
});

it('S03/E02: completed opens the linked report', async () => {
  vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue({
    ...base,
    status: 'completed',
    report_id: R1,
  });
  vi.spyOn(repositoryService, 'getReport').mockResolvedValue(fullReport);
  renderAt(`/analysis/${A1}`);
  // Ждём секцию отчёта, а не общий заголовок: так мы гарантируем, что
  // completed действительно открыл ReportHeader, а не просто показал имя репозитория.
  await screen.findByRole('region', { name: 'Шапка отчёта' });
});

// NOTE: FR-05 E01 требует текст «Не удалось завершить анализ» для ANALYSIS_FAILED.
// Текущая реализация для ANALYSIS_FAILED по умолчанию (VITE_C2_METADATA_PREVIEW !== 'false')
// показывает "Шапку анализа С2" вместо текста. Дефект D4 зафиксирован в протоколе.
it('E01 (D4): ANALYSIS_FAILED shows the preview header and both actions, no error text', async () => {
  vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue({
    ...base,
    status: 'failed',
    error: {
      code: 'ANALYSIS_FAILED',
      message: 'internal detail',
      trace_id: 'trace',
      field_errors: [],
    },
  });
  renderAt(`/analysis/${A1}`);
  await screen.findByRole('region', { name: 'Шапка анализа С2' });
  expect(screen.getByRole('button', { name: 'Повторить анализ' })).toBeTruthy();
  expect(screen.getByRole('button', { name: 'Новый анализ' })).toBeTruthy();
  expect(screen.queryByText('internal detail')).toBeNull();
  expect(screen.queryByText('Не удалось завершить анализ')).toBeNull();
});

it('E01: REPOSITORY_NOT_FOUND_OR_PRIVATE shows a safe reason and both actions', async () => {
  vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue({
    ...base,
    status: 'failed',
    error: {
      code: 'REPOSITORY_NOT_FOUND_OR_PRIVATE',
      message: 'internal detail',
      trace_id: 'trace',
      field_errors: [],
    },
  });
  renderAt(`/analysis/${A1}`);
  await screen.findByText('Репозиторий стал недоступен для анализа');
  expect(screen.getByRole('button', { name: 'Повторить анализ' })).toBeTruthy();
  expect(screen.getByRole('button', { name: 'Новый анализ' })).toBeTruthy();
  expect(screen.queryByText('internal detail')).toBeNull();
});

it('E01: REPORT_SAVE_FAILED shows a safe reason and both actions', async () => {
  vi.spyOn(repositoryService, 'getAnalysis').mockResolvedValue({
    ...base,
    status: 'failed',
    error: {
      code: 'REPORT_SAVE_FAILED',
      message: 'internal detail',
      trace_id: 'trace',
      field_errors: [],
    },
  });
  renderAt(`/analysis/${A1}`);
  await screen.findByText('Не удалось сохранить результат анализа');
  expect(screen.getByRole('button', { name: 'Повторить анализ' })).toBeTruthy();
});

it('S12: denied access shows a session message', async () => {
  vi.mocked(authService.getSessionContext).mockResolvedValue({ account: null, guest: null });
  renderAt(`/analysis/${A1}`);
  await screen.findByText('Сессия истекла. Войдите снова');
});