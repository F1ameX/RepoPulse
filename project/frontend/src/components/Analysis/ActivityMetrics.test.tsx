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
  message: 'Источник не ответил вовремя',
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
      series: [],
      evidence: [],
      evidence_limited: false,
      evidence_total: 0,
      observed_at: null,
      exclusions: [],
      ...overrides,
    },
    normalized_score: status === 'complete' ? 50 : null,
    rule_version: 'rhs-mvp-1.1.0',
    rule: '100 * share',
    threshold: null,
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

it('FR-17: unavailable + reason → N/A с безопасным cannot_conclude, без утечки message', () => {
  render(
    <ActivityMetrics
      report={makeReport([
        makeMetric('ACT_COMMIT_FREQ', null, 'unavailable', TIMEOUT_REASON),
        makeMetric('ACT_DAYS_SINCE_LAST', null, 'unavailable', TIMEOUT_REASON),
        makeMetric('ACT_TREND', null, 'unavailable', TIMEOUT_REASON),
      ])}
    />,
  );

  expect(screen.getAllByText('N/A')).toHaveLength(6);
  
  expect(screen.getAllByText(/Полная оценка этой проверки недоступна/).length).toBeGreaterThanOrEqual(3);
  
  expect(screen.queryByText('Источник не ответил вовремя')).toBeNull();
});

it('FR-17: evidence с небезопасным URL не рендерится как ссылка', () => {
  const safeEvidence: Evidence = {
    id: 'ev-safe',
    source: 'git',
    kind: 'commit',
    description: 'Safe commit',
    path: 'src/index.ts',
    revision: null,
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
    url: 'javascript:alert(1)',
  };
  const metric = makeMetric('ACT_COMMIT_FREQ', 3.14, 'complete', null, {
    evidence: [safeEvidence, unsafeEvidence],
    evidence_limited: true,
    evidence_total: 10,
  });
  render(<ActivityMetrics report={makeReport([metric])} />);

  const safeLink = screen.getByRole('link', { name: '[ссылка]' });
  expect(safeLink.getAttribute('href')).toBe(safeEvidence.url);
  expect(safeLink.getAttribute('target')).toBe('_blank');
  expect(safeLink.getAttribute('rel')).toContain('noopener');

  expect(screen.getByText('Unsafe commit')).toBeTruthy();
  const links = screen.getAllByRole('link', { name: '[ссылка]' });
  expect(links).toHaveLength(1);

  expect(screen.getByText('Показано 2 из 10')).toBeTruthy();
});

it('FR-07 (интеграция): окно и даты в UTC независимо от локали', () => {
  // Часовой пояс не влияет: formatUtc режет toISOString().
  render(
    <ActivityMetrics
      report={makeReport([makeMetric('ACT_COMMIT_FREQ', 5)])}
    />,
  );
  expect(
    screen.getByText('[02.06.2026 00:00:00 UTC, 20.10.2026 00:00:00 UTC)'),
  ).toBeTruthy();
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