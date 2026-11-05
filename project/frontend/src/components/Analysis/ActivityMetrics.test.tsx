import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, it } from 'vitest';
import type { Evidence, Metric, MetricId, Report, Reason } from '@/types';
import { ActivityMetrics } from './ActivityMetrics';

const repo = {
  platform: 'github' as const,
  github_id: 'example-only',
  owner: 'example',
  name: 'repopulse-fixture',
  canonical_url: 'https://github.com/example/repopulse-fixture',
  default_branch: 'main',
  commit_sha: '0000000000000000000000000000000000000000',
};

const TIMEOUT_REASON: Reason = {
  code: 'timeout',
  message: 'Знаменатель коэффициента тренда равен нулю',
  cannot_conclude: 'Полная оценка этой проверки недоступна',
};

function makeMetric(
  id: MetricId,
  raw: number | boolean | string | null,
  status: 'complete' | 'partial' | 'unavailable' | 'not_applicable' = 'complete',
  reason: Reason | null = null,
  overrides: Partial<Metric['input']> = {},
): Metric {
  return {
    input: {
      id,
      raw_value: raw,
      unit: 'ratio',
      status,
      reason,
      window:
        status === 'complete'
          ? { from: '2026-06-02T00:00:00Z', until: '2026-10-20T00:00:00Z' }
          : null,
      sample: {
        collected: status === 'complete' ? 5 : null,
        total: status === 'complete' ? 5 : null,
        numerator: null,
        denominator: null,
        excluded: null,
        excluded_reason: null,
      },
      series: status === 'complete' ? [1, 2, 3] : [],
      evidence: [],
      evidence_limited: false,
      evidence_total: 0,
      observed_at: '2026-10-20T00:00:00Z',
      exclusions: [],
      ...overrides,
    },
    normalized_score: status === 'complete' ? 50 : null,
    rule_version: 'rhs-mvp-1.1.0',
    rule: '100 * share',
    threshold: 10,
    explanation: '',
  };
}

function makeReport(metrics: Metric[]): Report {
  return {
    report_id: '00000000-0000-4000-8000-000000000004',
    analysis_id: '00000000-0000-4000-8000-000000000002',
    snapshot_id: '00000000-0000-4000-8000-000000000003',
    repository: repo,
    assessment_at: '2026-10-20T00:00:00Z',
    window: { from: '2026-06-02T00:00:00Z', until: '2026-10-20T00:00:00Z' },
    completed_at: '2026-10-20T00:00:00Z',
    versions: {} as Report['versions'],
    overall_score: 50,
    overall_reason: null,
    summary: '',
    coverage: {
      calculated: 2,
      total: 12,
      percentage: 16.7,
      partial: 0,
      unavailable: 10,
      not_applicable: 0,
    },
    categories: [
      {
        id: 'activity',
        score: null,
        status: 'unavailable',
        reason: TIMEOUT_REASON,
        coverage: {
          calculated: 0,
          total: 3,
          percentage: 0,
          partial: 0,
          unavailable: 3,
          not_applicable: 0,
        },
        effective_weight: null,
        explanation: '',
        metrics,
      },
    ],
    sources: [],
    dependencies: [],
    findings: [],
    recommendations: [],
    explanation: {} as Report['explanation'],
  };
}

afterEach(cleanup);

it('FR-10: форматирует сырые значения как в ТЗ — частота 2 знака, дни целое, тренд 4 знака', () => {
  render(
    <ActivityMetrics
      report={makeReport([
        makeMetric('ACT_COMMIT_FREQ', 12.3456),
        makeMetric('ACT_DAYS_SINCE_LAST', 42.9),
        makeMetric('ACT_TREND', 0.123456),
      ])}
    />,
  );

  expect(screen.getByText('12.35')).toBeTruthy();
  expect(screen.getByText('43')).toBeTruthy();
  expect(screen.getByText('0.1235')).toBeTruthy();
});

it('P2: округляет half-up (0.00015 -> 0.0002 и 2.00085 -> 2.0009)', () => {
  const { rerender } = render(
    <ActivityMetrics
      report={makeReport([makeMetric('ACT_TREND', 0.00015)])}
    />,
  );
  expect(screen.getByText('0.0002')).toBeTruthy();

  rerender(
    <ActivityMetrics
      report={makeReport([makeMetric('ACT_TREND', 2.00085)])}
    />,
  );
  expect(screen.getByText('2.0009')).toBeTruthy();
});

it('FR-17: 0 — валидное значение, а не N/A', () => {
  render(
    <ActivityMetrics
      report={makeReport([
        makeMetric('ACT_COMMIT_FREQ', 0),
        makeMetric('ACT_DAYS_SINCE_LAST', 0),
        makeMetric('ACT_TREND', 0),
      ])}
    />,
  );

  expect(screen.getByText('0.00')).toBeTruthy();
  expect(screen.getByText('0')).toBeTruthy();
  expect(screen.getByText('0.0000')).toBeTruthy();
  expect(screen.queryByText('N/A')).toBeNull();
});

it('P1: unavailable + reason → N/A с предметной причиной и общим выводом', () => {
  render(
    <ActivityMetrics
      report={makeReport([
        makeMetric('ACT_COMMIT_FREQ', null, 'unavailable', TIMEOUT_REASON),
      ])}
    />,
  );

  expect(screen.getAllByText('N/A')).toHaveLength(2);
  expect(
    screen.getByText(/Знаменатель коэффициента тренда равен нулю/),
  ).toBeTruthy();
  expect(
    screen.getAllByText(/Полная оценка этой проверки недоступна/).length,
  ).toBeGreaterThanOrEqual(1);
});

it('P1: выводит статус полноты (complete -> «Полные данные»)', () => {
  render(
    <ActivityMetrics
      report={makeReport([makeMetric('ACT_COMMIT_FREQ', 5, 'complete')])}
    />,
  );
  expect(screen.getByText('Полные данные')).toBeTruthy();
});

it('P1: evidence с небезопасным URL (userinfo bypass) не рендерится как ссылка', () => {
  const safeEvidence: Evidence = {
    id: 'ev-safe',
    source: 'git',
    kind: 'commit',
    description: 'Safe commit',
    path: 'src/index.ts',
    revision: 'abc123',
    url: 'https://github.com/example/repopulse-fixture/commit/abc',
    run_id: null,
    attempt: null,
    dependency: null,
    vulnerability_id: null,
    secret_match_type: null,
  };
  const unsafeEvidence: Evidence = {
    ...safeEvidence,
    id: 'ev-unsafe',
    description: 'Unsafe commit',
    url: 'https://github.com@evil.example/',
  };
  const missingEvidence: Evidence = {
    ...safeEvidence,
    id: 'ev-missing',
    description: 'Missing URL commit',
    url: null,
  };

  const metric = makeMetric('ACT_COMMIT_FREQ', 3.14, 'complete', null, {
    evidence: [safeEvidence, unsafeEvidence, missingEvidence],
    evidence_limited: true,
    evidence_total: 10,
  });
  render(<ActivityMetrics report={makeReport([metric])} />);

  expect(screen.getAllByText(/source: git/).length).toBe(3);
  expect(screen.getAllByText(/rev: abc123/).length).toBe(3);

  const safeLink = screen.getByRole('link', { name: '[ссылка]' });
  expect(safeLink.getAttribute('href')).toBe(safeEvidence.url);
  expect(safeLink.getAttribute('target')).toBe('_blank');
  expect(safeLink.getAttribute('rel')).toContain('noopener');

  expect(screen.getByText('Unsafe commit')).toBeTruthy();
  expect(screen.getByText('Missing URL commit')).toBeTruthy();

  expect(screen.getAllByRole('link', { name: '[ссылка]' })).toHaveLength(1);

  expect(screen.getAllByText(/Ссылка на источник недоступна/)).toHaveLength(2);
});

it('P1: пустой evidence выводит явное сообщение', () => {
  const metric = makeMetric('ACT_COMMIT_FREQ', 5, 'complete', null, {
    evidence: [],
  });
  render(<ActivityMetrics report={makeReport([metric])} />);
  expect(screen.getByText('Основания не сохранены')).toBeTruthy();
});

it('FR-07/P1 (интеграция): окно, серия и дата наблюдения в UTC', () => {
  render(
    <ActivityMetrics report={makeReport([makeMetric('ACT_COMMIT_FREQ', 5)])} />,
  );
  expect(
    screen.getByText(
      '[02.06.2026 00:00:00 UTC, 20.10.2026 00:00:00 UTC)',
    ),
  ).toBeTruthy();
  expect(screen.getByText('1, 2, 3')).toBeTruthy();
  expect(screen.getByText('20.10.2026 00:00:00 UTC')).toBeTruthy();
});

it('возвращает null, если в отчёте нет категории activity', () => {
  const { container } = render(
    <ActivityMetrics
      report={{
        ...makeReport([]),
        categories: [],
      }}
    />,
  );
  expect(container.firstChild).toBeNull();
});