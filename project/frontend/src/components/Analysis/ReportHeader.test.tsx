import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, it } from 'vitest';
import type { Repository } from '@/types';
import { ReportHeader } from './ReportHeader';

const repository: Repository = {
  platform: 'github',
  github_id: '1',
  owner: 'owner',
  name: 'repo',
  canonical_url: 'https://github.com/owner/repo',
  default_branch: null,
  commit_sha: null,
};
const value = (label: string) => screen.getByText(`${label}:`).nextElementSibling!;
afterEach(cleanup);

it('keeps unknown metadata and versions visible without inventing dates or a commit link', () => {
  render(<ReportHeader report={{ repository, analysis_id: 'analysis-1' }} />);
  for (const label of [
    'Завершён',
    'Момент оценки',
    'Период оценки',
    'Ветка',
    'Commit',
    'Методика',
    'Профиль сборщика',
    'Каталог рекомендаций',
  ]) {
    expect(value(label).textContent).toBe('Не определено');
    expect(value(label).querySelector('a')).toBeNull();
  }
  expect(screen.queryByText('Отчёт:')).toBeNull();
});

it('preserves known snapshot metadata, full IDs and SHA, with UTC times and safe links', () => {
  const sha = 'a'.repeat(40);
  render(
    <ReportHeader
      report={{
        repository: { ...repository, default_branch: 'main', commit_sha: sha },
        analysis_id: 'analysis-1',
        report_id: 'report-1',
        completed_at: '2026-10-29T17:00:00+07:00',
        assessment_at: '2026-10-29T09:00:00Z',
        window: { from: '2026-10-01T00:00:00Z', until: '2026-10-29T09:00:00Z' },
        versions: { methodology: 'v1' },
      }}
    />,
  );
  expect(value('Анализ').textContent).toBe('analysis-1');
  expect(value('Отчёт').textContent).toBe('report-1');
  expect(value('Завершён').textContent).toBe('29.10.2026 10:00:00 UTC');
  expect(value('Период оценки').textContent).toBe(
    '[01.10.2026 00:00:00 UTC, 29.10.2026 09:00:00 UTC)',
  );
  expect(value('Ветка').textContent).toBe('main');
  expect(value('Методика').textContent).toBe('v1');
  const commit = screen.getByRole('link', { name: sha });
  expect(commit.getAttribute('href')).toBe(`${repository.canonical_url}/commit/${sha}`);
  expect(commit.getAttribute('target')).toBe('_blank');
});

it('omits a duplicate report ID and does not link an unknown or invalid SHA', () => {
  render(
    <ReportHeader
      report={{
        repository: { ...repository, default_branch: 'Не определен', commit_sha: '' },
        analysis_id: 'analysis-1',
        report_id: 'analysis-1',
        window: { from: '', until: '2026-10-29T09:00:00Z' },
      }}
    />,
  );
  expect(screen.queryByText('Отчёт:')).toBeNull();
  expect(value('Ветка').textContent).toBe('Не определено');
  expect(value('Commit').textContent).toBe('Не определено');
  expect(value('Период оценки').textContent).toBe('Не определено');
  expect(value('Commit').querySelector('a')).toBeNull();
});
